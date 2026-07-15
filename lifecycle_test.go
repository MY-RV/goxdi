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

func TestNestedScopeDoesNotInheritScoped(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{name: "root"}, nil
	}))
	mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*repo, error) {
		return &repo{}, nil
	}))
	root := b.Build()
	defer root.Close()

	parent := root.NewScope()
	defer parent.Close()
	child := parent.NewScope()
	defer child.Close()

	if goxdi.MustGet[*repo](parent) == goxdi.MustGet[*repo](child) {
		t.Fatal("nested scope must not inherit parent scoped instances")
	}
	if goxdi.MustGet[*db](parent) != goxdi.MustGet[*db](child) {
		t.Fatal("nested scope must still share root singletons")
	}
}

func TestDoubleCloseReturnsErrClosed(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{}, nil
	}))
	root := b.Build()

	scope := root.NewScope()
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if err := scope.Close(); !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("second Scope.Close: got %v, want ErrClosed", err)
	}

	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("second Container.Close: got %v, want ErrClosed", err)
	}
}

func TestClosedContainerRejectsResolve(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{}, nil
	}))
	root := b.Build()
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := goxdi.Get[*db](root)
	if !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("got %v, want ErrClosed", err)
	}
}

func TestClosedRootRejectsResolveFromOpenScope(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		return &db{}, nil
	}))
	root := b.Build()
	scope := root.NewScope()
	defer scope.Close()

	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := goxdi.Get[*db](scope)
	if !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("got %v, want ErrClosed", err)
	}
}

type countingCloser struct {
	n *atomic.Int32
}

func (c *countingCloser) Close() error {
	c.n.Add(1)
	return nil
}

type failCloser struct {
	n   *atomic.Int32
	err error
}

func (c *failCloser) Close() error {
	c.n.Add(1)
	return c.err
}

type failCloserB struct {
	n   *atomic.Int32
	err error
}

func (c *failCloserB) Close() error {
	c.n.Add(1)
	return c.err
}

func TestCloseReturnsFirstDisposerError(t *testing.T) {
	firstErr := errors.New("first")
	secondErr := errors.New("second")
	var aCalls, bCalls atomic.Int32
	a := &failCloser{n: &aCalls, err: firstErr}
	bb := &failCloserB{n: &bCalls, err: secondErr}

	builder := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(builder, func(goxdi.Resolver) (*failCloser, error) {
		return a, nil
	}))
	mustAdd(t, goxdi.AddSingleton(builder, func(goxdi.Resolver) (*failCloserB, error) {
		return bb, nil
	}))
	root := builder.Build()
	_ = goxdi.MustGet[*failCloser](root)
	_ = goxdi.MustGet[*failCloserB](root)

	err := root.Close()
	// LIFO: failCloserB closes first; its error is returned.
	if !errors.Is(err, secondErr) {
		t.Fatalf("got %v, want %v", err, secondErr)
	}
	if aCalls.Load() != 1 || bCalls.Load() != 1 {
		t.Fatalf("both closers should run: a=%d b=%d", aCalls.Load(), bCalls.Load())
	}
}

func TestMustGetPanics(t *testing.T) {
	root := goxdi.NewBuilder().Build()
	defer root.Close()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		err, ok := r.(error)
		if !ok || !errors.Is(err, goxdi.ErrNotRegistered) {
			t.Fatalf("panic value: %#v", r)
		}
	}()
	_ = goxdi.MustGet[*db](root)
}

func TestSingletonFactoryPanicUnblocksWaiters(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	var panicked atomic.Bool

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		closeOnce(&startOnce, started)
		<-release
		if panicked.CompareAndSwap(false, true) {
			panic("factory boom")
		}
		// Late Gets after the flight cleared may re-enter; return an error instead of re-panicking.
		return nil, errors.New("factory already panicked")
	}))
	root := b.Build()
	defer root.Close()

	leaderErr := make(chan any, 1)
	go func() {
		defer func() { leaderErr <- recover() }()
		_ = goxdi.MustGet[*db](root)
	}()

	<-started

	var wg sync.WaitGroup
	waiterErrs := make([]error, 4)
	for i := range waiterErrs {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			_, waiterErrs[i] = goxdi.Get[*db](root)
		}()
	}

	waitForSchedules(t, 64)
	close(release)

	if recoverVal := <-leaderErr; recoverVal == nil {
		t.Fatal("leader should panic")
	}
	wg.Wait()

	for i, err := range waiterErrs {
		if err == nil {
			t.Fatalf("waiter %d: expected error after factory panic", i)
		}
	}
}

func TestConcurrentSingletonWaitersSeeLeaderError(t *testing.T) {
	boom := errors.New("build failed")
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		closeOnce(&startOnce, started)
		<-release
		return nil, boom
	}))
	root := b.Build()
	defer root.Close()

	const n = 6
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, errs[i] = goxdi.Get[*db](root)
		}()
	}

	<-started
	waitForSchedules(t, 64)
	close(release)
	wg.Wait()

	for i, err := range errs {
		if !errors.Is(err, boom) {
			t.Fatalf("goroutine %d: got %v, want %v", i, err, boom)
		}
	}
}

func TestConcurrentScopedCreateClosesLoser(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var builds atomic.Int32
	var closed atomic.Int32

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*countingCloser, error) {
		builds.Add(1)
		started <- struct{}{}
		<-release
		return &countingCloser{n: &closed}, nil
	}))
	root := b.Build()
	defer root.Close()

	scope := root.NewScope()
	defer scope.Close()

	results := make([]*countingCloser, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i], errs[i] = goxdi.Get[*countingCloser](scope)
		}()
	}

	<-started
	<-started
	close(release)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	if builds.Load() != 2 {
		t.Fatalf("expected 2 concurrent scoped builds, got %d", builds.Load())
	}
	if results[0] != results[1] {
		t.Fatal("both Gets should observe the same scoped winner")
	}
	if closed.Load() != 1 {
		t.Fatalf("expected exactly one orphan Close, got %d", closed.Load())
	}
}

func TestTransientCloserTrackedOnRoot(t *testing.T) {
	var closed atomic.Int32
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddTransient(b, func(goxdi.Resolver) (*countingCloser, error) {
		return &countingCloser{n: &closed}, nil
	}))
	root := b.Build()

	_ = goxdi.MustGet[*countingCloser](root)
	_ = goxdi.MustGet[*countingCloser](root)
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if closed.Load() != 2 {
		t.Fatalf("expected 2 transient closers disposed, got %d", closed.Load())
	}
}

func TestCircularScopedDependency(t *testing.T) {
	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddScoped(b, func(r goxdi.Resolver) (*circA, error) {
		dep, err := goxdi.Get[*circB](r)
		if err != nil {
			return nil, err
		}
		return &circA{b: dep}, nil
	}))
	mustAdd(t, goxdi.AddScoped(b, func(r goxdi.Resolver) (*circB, error) {
		dep, err := goxdi.Get[*circA](r)
		if err != nil {
			return nil, err
		}
		return &circB{a: dep}, nil
	}))
	root := b.Build()
	defer root.Close()

	scope := root.NewScope()
	defer scope.Close()

	_, err := goxdi.Get[*circA](scope)
	if !errors.Is(err, goxdi.ErrCircularDependency) {
		t.Fatalf("expected circular dependency, got %v", err)
	}
}

func TestTransientAbortedWhenContainerClosedDuringFactory(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	var closed atomic.Int32

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddTransient(b, func(goxdi.Resolver) (*countingCloser, error) {
		closeOnce(&startOnce, started)
		<-release
		return &countingCloser{n: &closed}, nil
	}))
	root := b.Build()

	errCh := make(chan error, 1)
	go func() {
		_, err := goxdi.Get[*countingCloser](root)
		errCh <- err
	}()

	<-started
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	close(release)

	err := <-errCh
	if !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("got %v, want ErrClosed", err)
	}
	if closed.Load() != 1 {
		t.Fatalf("aborted transient should be closed, got %d", closed.Load())
	}
}

func TestScopedCreateAbortedWhenScopeClosedDuringFactory(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	var closed atomic.Int32

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddScoped(b, func(goxdi.Resolver) (*countingCloser, error) {
		closeOnce(&startOnce, started)
		<-release
		return &countingCloser{n: &closed}, nil
	}))
	root := b.Build()
	defer root.Close()

	scope := root.NewScope()
	errCh := make(chan error, 1)
	go func() {
		_, err := goxdi.Get[*countingCloser](scope)
		errCh <- err
	}()

	<-started
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	close(release)

	err := <-errCh
	if !errors.Is(err, goxdi.ErrClosed) {
		t.Fatalf("got %v, want ErrClosed", err)
	}
	if closed.Load() != 1 {
		t.Fatalf("aborted instance should be closed, got %d", closed.Load())
	}
}

func TestAwaitInFlightSeesClosedAfterSuccess(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once

	b := goxdi.NewBuilder()
	mustAdd(t, goxdi.AddSingleton(b, func(goxdi.Resolver) (*db, error) {
		closeOnce(&startOnce, started)
		<-release
		return &db{name: "x"}, nil
	}))
	root := b.Build()

	leaderDone := make(chan error, 1)
	go func() {
		_, err := goxdi.Get[*db](root)
		leaderDone <- err
	}()
	<-started

	waiterDone := make(chan error, 1)
	go func() {
		_, err := goxdi.Get[*db](root)
		waiterDone <- err
	}()
	waitForSchedules(t, 64)

	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	close(release)

	leaderErr := <-leaderDone
	waiterErr := <-waiterDone
	if !errors.Is(leaderErr, goxdi.ErrClosed) {
		t.Fatalf("leader: got %v, want ErrClosed", leaderErr)
	}
	if !errors.Is(waiterErr, goxdi.ErrClosed) {
		t.Fatalf("waiter: got %v, want ErrClosed", waiterErr)
	}
}

// waitForSchedules yields so peer goroutines can reach awaitInFlight / create.
func waitForSchedules(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for i := 0; i < n; i++ {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for scheduler progress")
		}
		runtime.Gosched()
	}
}

// closeOnce closes ch at most once (singleton factory may retry after a failed flight).
func closeOnce(once *sync.Once, ch chan struct{}) {
	once.Do(func() { close(ch) })
}
