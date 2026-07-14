package goxdi_test

import (
	"errors"
	"testing"

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
	if !errors.Is(err, goxdi.ErrScopeClosed) {
		t.Fatalf("expected ErrScopeClosed, got %v", err)
	}
	_ = root.Close()
}
