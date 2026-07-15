package container

import (
	"fmt"
	"io"
	"reflect"
	"sync"
)

type definition struct {
	lifetime    Lifetime
	serviceType reflect.Type
	create      func(Resolver) (any, error)
}

// inFlight is one in-progress Singleton construction (singleflight).
// Concurrent Get calls for the same type wait on done and share instance/err.
type inFlight struct {
	done     chan struct{}
	instance any
	err      error
}

func newInFlight() *inFlight {
	return &inFlight{done: make(chan struct{})}
}

// Container is the root DI container. It owns Singleton instances and creates
// scopes for Scoped/Transient lifetimes.
//
// Concurrency: the root mutex serializes cache/disposer updates. Factories run
// without holding that mutex. Concurrent first-time Get of the same Singleton
// is singleflighted — one factory runs; others wait and receive the same result.
// Prefer one Scope per unit of work; do not share a Scope across goroutines.
type Container struct {
	defs       map[reflect.Type]*definition
	singletons map[reflect.Type]any
	inflight   map[reflect.Type]*inFlight
	mu         sync.Mutex
	disposers  []io.Closer
	closed     bool
}

func newContainer(defs map[reflect.Type]*definition) *Container {
	return &Container{
		defs:       defs,
		singletons: make(map[reflect.Type]any),
		inflight:   make(map[reflect.Type]*inFlight),
	}
}

// NewScope returns a child scope that shares Singletons with the root.
func (c *Container) NewScope() *Scope {
	return &Scope{
		root:   c,
		scoped: make(map[reflect.Type]any),
	}
}

// Close disposes Singleton (and root-resolved Transient) instances that implement io.Closer.
// After Close, further resolves return ErrClosed.
func (c *Container) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrClosed
	}
	c.closed = true
	return closeAll(c.disposers)
}

func (c *Container) resolve(serviceType reflect.Type) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, ErrClosed
	}
	return c.resolveLocked(serviceType, nil, nil)
}

func (c *Container) resolveLocked(serviceType reflect.Type, scope *Scope, stack []reflect.Type) (any, error) {
	def, ok := c.defs[serviceType]
	if !ok {
		return nil, errNotRegistered(serviceType)
	}

	switch def.lifetime {
	case Singleton:
		return c.resolveSingleton(def, stack)

	case Scoped:
		if scope == nil {
			return nil, fmt.Errorf("%w: %s", ErrScopeRequired, serviceType)
		}
		if instance, ok := scope.scoped[serviceType]; ok {
			return instance, nil
		}
		instance, err := c.create(def, scope, stack)
		if err != nil {
			return nil, err
		}
		if c.closed || scope.closed {
			closeIfCloser(instance)
			return nil, ErrClosed
		}
		if existing, ok := scope.scoped[serviceType]; ok {
			closeOrphan(instance, existing)
			return existing, nil
		}
		scope.scoped[serviceType] = instance
		trackCloser(&scope.disposers, instance)
		return instance, nil

	case Transient:
		instance, err := c.create(def, scope, stack)
		if err != nil {
			return nil, err
		}
		if c.closed || (scope != nil && scope.closed) {
			closeIfCloser(instance)
			return nil, ErrClosed
		}
		if scope != nil {
			trackCloser(&scope.disposers, instance)
		} else {
			trackCloser(&c.disposers, instance)
		}
		return instance, nil

	default:
		return nil, fmt.Errorf("invalid lifetime for %s", serviceType)
	}
}

// resolveSingleton enforces: at most one factory execution per type; waiters
// join the in-flight call. Construction always uses a nil scope (no captive Scoped deps).
func (c *Container) resolveSingleton(def *definition, stack []reflect.Type) (any, error) {
	serviceType := def.serviceType
	if instance, ok := c.singletons[serviceType]; ok {
		return instance, nil
	}
	if flight, ok := c.inflight[serviceType]; ok {
		if containsType(stack, serviceType) {
			return nil, errCircular(stack, serviceType)
		}
		return c.awaitInFlight(flight)
	}

	flight := newInFlight()
	c.inflight[serviceType] = flight
	// If the factory panics, finishInFlight was never called — unblock waiters.
	defer c.finishInFlightIfOpen(serviceType, flight, errFactoryPanicked)

	instance, err := c.create(def, nil, stack)
	if err != nil {
		c.finishInFlight(serviceType, flight, nil, err)
		return nil, err
	}
	if c.closed {
		closeIfCloser(instance)
		c.finishInFlight(serviceType, flight, nil, ErrClosed)
		return nil, ErrClosed
	}

	c.singletons[serviceType] = instance
	trackCloser(&c.disposers, instance)
	c.finishInFlight(serviceType, flight, instance, nil)
	return instance, nil
}

// finishInFlight publishes the singleflight result and unblocks waiters.
// Caller must hold c.mu. flight must still be registered in c.inflight.
func (c *Container) finishInFlight(serviceType reflect.Type, flight *inFlight, instance any, err error) {
	flight.instance = instance
	flight.err = err
	delete(c.inflight, serviceType)
	close(flight.done)
}

// finishInFlightIfOpen runs only when finishInFlight was skipped (e.g. panic).
func (c *Container) finishInFlightIfOpen(serviceType reflect.Type, flight *inFlight, err error) {
	if _, ok := c.inflight[serviceType]; !ok {
		return
	}
	c.finishInFlight(serviceType, flight, nil, err)
}

func (c *Container) awaitInFlight(flight *inFlight) (any, error) {
	c.mu.Unlock()
	<-flight.done
	c.mu.Lock()

	if flight.err != nil {
		return nil, flight.err
	}
	if c.closed {
		return nil, ErrClosed
	}
	return flight.instance, nil
}

// create runs def.create. Caller must hold c.mu; the mutex is released while the
// factory runs (nested Get via the factory Resolver reacquires it) and held
// again on return, including after a panic in the factory.
func (c *Container) create(def *definition, scope *Scope, stack []reflect.Type) (any, error) {
	serviceType := def.serviceType
	if containsType(stack, serviceType) {
		return nil, errCircular(stack, serviceType)
	}

	next := make([]reflect.Type, len(stack)+1)
	copy(next, stack)
	next[len(stack)] = serviceType

	ctx := &resolveContext{root: c, scope: scope, stack: next}

	c.mu.Unlock()
	defer c.mu.Lock()
	return def.create(ctx)
}

func containsType(stack []reflect.Type, t reflect.Type) bool {
	for _, s := range stack {
		if s == t {
			return true
		}
	}
	return false
}

func trackCloser(list *[]io.Closer, instance any) {
	if closer, ok := instance.(io.Closer); ok {
		*list = append(*list, closer)
	}
}

func closeIfCloser(instance any) {
	if closer, ok := instance.(io.Closer); ok {
		_ = closer.Close()
	}
}

// closeOrphan closes instance when a concurrent Scoped create lost the race.
func closeOrphan(instance, existing any) {
	if instance == existing {
		return
	}
	closeIfCloser(instance)
}

func closeAll(closers []io.Closer) error {
	var first error
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i].Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
