package container

import (
	"reflect"
	"testing"
)

type stubResolver struct {
	raw any
	err error
}

func (s stubResolver) resolve(reflect.Type) (any, error) {
	return s.raw, s.err
}

func TestGetNotAssignable(t *testing.T) {
	_, err := Get[string](stubResolver{raw: 42})
	if err == nil {
		t.Fatal("expected assignability error")
	}
	want := "resolved int is not assignable to string"
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestCloseOrphan(t *testing.T) {
	closed := false
	loser := &testCloser{closed: &closed}
	winner := &testCloser{}

	closeOrphan(loser, winner)
	if !closed {
		t.Fatal("loser closer should run")
	}

	closed = false
	same := &testCloser{closed: &closed}
	closeOrphan(same, same)
	if closed {
		t.Fatal("identical instance must not be closed as orphan")
	}
}

type testCloser struct {
	closed *bool
}

func (c *testCloser) Close() error {
	if c.closed != nil {
		*c.closed = true
	}
	return nil
}

func TestResolveInvalidLifetime(t *testing.T) {
	serviceType := typeOf[*struct{}]()
	c := &Container{
		defs: map[reflect.Type]*definition{
			serviceType: {
				lifetime:    Lifetime(99),
				serviceType: serviceType,
				create:      func(Resolver) (any, error) { return &struct{}{}, nil },
			},
		},
		singletons: make(map[reflect.Type]any),
		inflight:   make(map[reflect.Type]*inFlight),
	}

	_, err := c.resolve(serviceType)
	if err == nil {
		t.Fatal("expected invalid lifetime error")
	}
}

func TestResolveContextRejectsClosedScope(t *testing.T) {
	serviceType := typeOf[*struct{}]()
	root := &Container{
		defs: map[reflect.Type]*definition{
			serviceType: {
				lifetime:    Transient,
				serviceType: serviceType,
				create:      func(Resolver) (any, error) { return &struct{}{}, nil },
			},
		},
		singletons: make(map[reflect.Type]any),
		inflight:   make(map[reflect.Type]*inFlight),
	}
	scope := &Scope{root: root, scoped: make(map[reflect.Type]any), closed: true}
	ctx := &resolveContext{root: root, scope: scope, stack: nil}

	_, err := ctx.resolve(serviceType)
	if err != ErrClosed {
		t.Fatalf("got %v, want ErrClosed", err)
	}
}

func TestResolveContextRejectsClosedRoot(t *testing.T) {
	serviceType := typeOf[*struct{}]()
	root := &Container{
		defs: map[reflect.Type]*definition{
			serviceType: {
				lifetime:    Transient,
				serviceType: serviceType,
				create:      func(Resolver) (any, error) { return &struct{}{}, nil },
			},
		},
		singletons: make(map[reflect.Type]any),
		inflight:   make(map[reflect.Type]*inFlight),
		closed:     true,
	}
	ctx := &resolveContext{root: root, scope: nil, stack: nil}

	_, err := ctx.resolve(serviceType)
	if err != ErrClosed {
		t.Fatalf("got %v, want ErrClosed", err)
	}
}

func TestAwaitInFlightWhenClosedAfterSuccess(t *testing.T) {
	c := &Container{closed: true}
	flight := newInFlight()
	flight.instance = "done"
	close(flight.done)

	c.mu.Lock()
	got, err := c.awaitInFlight(flight)
	c.mu.Unlock()

	if err != ErrClosed {
		t.Fatalf("got err %v, want ErrClosed", err)
	}
	if got != nil {
		t.Fatalf("got instance %v, want nil", got)
	}
}
