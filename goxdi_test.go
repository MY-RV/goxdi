package goxdi_test

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MY-RV/goxdi"
)

type db struct{ name string }

type repo struct{ db *db }

type service struct{ repo *repo }

func mustAdd(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSingletonSharedAcrossScopes(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{name: "main"}, nil
	}))
	root := b.Build()
	defer root.Close()

	s1 := root.NewScope()
	defer s1.Close()
	s2 := root.NewScope()
	defer s2.Close()

	a := goxdi.MustGet[*db](s1)
	bb := goxdi.MustGet[*db](s2)
	c := goxdi.MustGet[*db](root)

	if a != bb || a != c {
		t.Fatalf("expected same singleton instance")
	}
}

func TestScopedUniquePerScope(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*db, error) {
		return &db{name: "scoped"}, nil
	}))
	root := b.Build()
	defer root.Close()

	s1 := root.NewScope()
	defer s1.Close()
	s2 := root.NewScope()
	defer s2.Close()

	a1 := goxdi.MustGet[*db](s1)
	a2 := goxdi.MustGet[*db](s1)
	bb := goxdi.MustGet[*db](s2)

	if a1 != a2 {
		t.Fatalf("scoped should reuse within scope")
	}
	if a1 == bb {
		t.Fatalf("scoped must differ across scopes")
	}
}

func TestScopedRequiresScope(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*db, error) {
		return &db{}, nil
	}))
	root := b.Build()
	defer root.Close()

	_, err := goxdi.Get[*db](root)
	if !errors.Is(err, goxdi.ErrScopeRequired) {
		t.Fatalf("expected ErrScopeRequired, got %v", err)
	}
}

func TestTransientAlwaysNew(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddTransient(b, func(goxdi.Resolver) (*db, error) {
		return &db{}, nil
	}))
	root := b.Build()
	defer root.Close()

	scope := root.NewScope()
	defer scope.Close()

	a := goxdi.MustGet[*db](scope)
	bb := goxdi.MustGet[*db](scope)
	if a == bb {
		t.Fatalf("transient should create new instances")
	}
}

func TestDependencyInjection(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{name: "db"}, nil
	}))
	mustAdd(t, goxdi.AddScoped(b, func(r goxdi.Resolver) (*repo, error) {
		return &repo{db: goxdi.MustGet[*db](r)}, nil
	}))
	mustAdd(t, goxdi.AddTransient(b, func(r goxdi.Resolver) (*service, error) {
		return &service{repo: goxdi.MustGet[*repo](r)}, nil
	}))
	root := b.Build()
	defer root.Close()

	scope := root.NewScope()
	defer scope.Close()

	svc := goxdi.MustGet[*service](scope)
	if svc.repo == nil || svc.repo.db == nil || svc.repo.db.name != "db" {
		t.Fatalf("dependencies not wired: %#v", svc)
	}
}

func TestFactoryReturningError(t *testing.T) {
	boom := errors.New("boom")

	t.Run("singleton", func(t *testing.T) {
		b := goxdi.NewBuilder()
		mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
			return nil, boom
		}))
		root := b.Build()
		defer root.Close()

		_, err := goxdi.Get[*db](root)
		if !errors.Is(err, boom) {
			t.Fatalf("expected boom, got %v", err)
		}
	})

	t.Run("scoped", func(t *testing.T) {
		b := goxdi.NewBuilder()
		mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*db, error) {
			return nil, boom
		}))
		root := b.Build()
		defer root.Close()
		scope := root.NewScope()
		defer scope.Close()

		_, err := goxdi.Get[*db](scope)
		if !errors.Is(err, boom) {
			t.Fatalf("expected boom, got %v", err)
		}
	})

	t.Run("transient", func(t *testing.T) {
		b := goxdi.NewBuilder()
		mustAdd(t, goxdi.AddTransient(b, func(goxdi.Resolver) (*db, error) {
			return nil, boom
		}))
		root := b.Build()
		defer root.Close()

		_, err := goxdi.Get[*db](root)
		if !errors.Is(err, boom) {
			t.Fatalf("expected boom, got %v", err)
		}
	})
}

type circA struct{ b *circB }
type circB struct{ a *circA }

func TestCircularDependency(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(r goxdi.Resolver) (*circA, error) {
		dep, err := goxdi.Get[*circB](r)
		if err != nil {
			return nil, err
		}
		return &circA{b: dep}, nil
	}))
	mustAdd(t, goxdi.AddSingleton(b, func(r goxdi.Resolver) (*circB, error) {
		dep, err := goxdi.Get[*circA](r)
		if err != nil {
			return nil, err
		}
		return &circB{a: dep}, nil
	}))
	root := b.Build()
	defer root.Close()

	_, err := goxdi.Get[*circA](root)
	if !errors.Is(err, goxdi.ErrCircularDependency) {
		t.Fatalf("expected circular dependency, got %v", err)
	}
}

func TestMissingDependencyAtResolve(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(r goxdi.Resolver) (*repo, error) {
		db, err := goxdi.Get[*db](r)
		if err != nil {
			return nil, err
		}
		return &repo{db: db}, nil
	}))
	root := b.Build()
	defer root.Close()

	_, err := goxdi.Get[*repo](root)
	if !errors.Is(err, goxdi.ErrNotRegistered) {
		t.Fatalf("expected ErrNotRegistered, got %v", err)
	}
}

func TestAddDuplicate(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{name: "first"}, nil
	}))
	err := goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{name: "second"}, nil
	})
	if !errors.Is(err, goxdi.ErrAlreadyRegistered) {
		t.Fatalf("expected ErrAlreadyRegistered, got %v", err)
	}

	root := b.Build()
	defer root.Close()
	if goxdi.MustGet[*db](root).name != "first" {
		t.Fatalf("duplicate add should leave original registration")
	}
}

type closerDB struct {
	closed *bool
}

func (c *closerDB) Close() error {
	*c.closed = true
	return nil
}

func TestScopeDisposesScopedClosers(t *testing.T) {
	closed := false
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*closerDB, error) {
		return &closerDB{closed: &closed}, nil
	}))
	root := b.Build()
	defer root.Close()

	scope := root.NewScope()
	_ = goxdi.MustGet[*closerDB](scope)
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatalf("expected scoped closer to run")
	}
}

type Store interface{ Name() string }

type memStore struct{}

func (memStore) Name() string { return "mem" }

func TestInterfaceRegistration(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (Store, error) {
		return memStore{}, nil
	}))
	root := b.Build()
	defer root.Close()

	store := goxdi.MustGet[Store](root)
	if store.Name() != "mem" {
		t.Fatalf("got %q", store.Name())
	}
}

func TestClosedScopeRejectsResolve(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{}, nil
	}))
	root := b.Build()

	scope := root.NewScope()
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := goxdi.Get[*db](scope)
	if !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if !errors.Is(err, goxdi.ErrScopeClosed) {
		t.Fatalf("ErrScopeClosed should alias ErrClosed, got %v", err)
	}
	_ = root.Close()
}

func TestContainerDisposesSingletonClosers(t *testing.T) {
	closed := false
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*closerDB, error) {
		return &closerDB{closed: &closed}, nil
	}))
	root := b.Build()

	_ = goxdi.MustGet[*closerDB](root)
	if closed {
		t.Fatalf("singleton closer must not run before Container.Close")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatalf("expected singleton closer to run on Container.Close")
	}
}

type captiveApp struct{ uow *closerDB }

func TestSingletonCannotCaptureScoped(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*closerDB, error) {
		closed := false
		return &closerDB{closed: &closed}, nil
	}))
	mustAdd(t, goxdi.AddSingleton(b, func(r goxdi.Resolver) (*captiveApp, error) {
		uow, err := goxdi.Get[*closerDB](r)
		if err != nil {
			return nil, err
		}
		return &captiveApp{uow: uow}, nil
	}))
	root := b.Build()
	defer root.Close()

	scope := root.NewScope()
	defer scope.Close()

	_, err := goxdi.Get[*captiveApp](scope)
	if !errors.Is(err, goxdi.ErrScopeRequired) {
		t.Fatalf("expected ErrScopeRequired for singleton→scoped, got %v", err)
	}
}

func TestFactoryResolveViaContainerDoesNotDeadlock(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{name: "db"}, nil
	}))
	var root *goxdi.Container
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*repo, error) {
		return &repo{db: goxdi.MustGet[*db](root)}, nil
	}))
	root = b.Build()
	defer root.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = goxdi.MustGet[*repo](root)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: factory Resolve via Container blocked")
	}
}

func TestConcurrentSingletonSingleflight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	var builds atomic.Int32

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		builds.Add(1)
		closeOnce(&startOnce, started)
		<-release
		return &db{name: "one"}, nil
	}))
	root := b.Build()
	defer root.Close()

	const n = 8
	results := make([]*db, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i], errs[i] = goxdi.Get[*db](root)
		}()
	}

	<-started
	// Yield so peers park in awaitInFlight while the leader is still constructing.
	for i := 0; i < 64; i++ {
		runtime.Gosched()
	}
	close(release)
	wg.Wait()

	if builds.Load() != 1 {
		t.Fatalf("factory ran %d times, want 1", builds.Load())
	}
	var first *db
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if first == nil {
			first = results[i]
			continue
		}
		if results[i] != first {
			t.Fatalf("goroutine %d got different instance", i)
		}
	}
}

func TestResolveAbortsIfClosedDuringCreate(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	closedFlag := false

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*closerDB, error) {
		closeOnce(&startOnce, started)
		<-release
		return &closerDB{closed: &closedFlag}, nil
	}))
	root := b.Build()

	errCh := make(chan error, 1)
	go func() {
		_, err := goxdi.Get[*closerDB](root)
		errCh <- err
	}()

	<-started
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	close(release)

	err := <-errCh
	if !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	if !closedFlag {
		t.Fatalf("orphaned instance created during Close should be closed")
	}
}
