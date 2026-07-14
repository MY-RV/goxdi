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

// Container is the root DI container. It owns singleton instances
// and creates scopes for scoped/transient lifetimes.
type Container struct {
	defs       map[reflect.Type]*definition
	singletons map[reflect.Type]any
	mu         sync.Mutex
	disposers  []io.Closer
	closed     bool
	resolving  []reflect.Type
}

func newContainer(defs map[reflect.Type]*definition) *Container {
	return &Container{
		defs:       defs,
		singletons: make(map[reflect.Type]any),
	}
}

// NewScope returns a child scope that shares singletons with the root.
func (c *Container) NewScope() *Scope {
	return &Scope{
		root:   c,
		scoped: make(map[reflect.Type]any),
	}
}

// Close disposes singleton instances created by this container that implement io.Closer.
func (c *Container) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrScopeClosed
	}
	c.closed = true
	return closeAll(c.disposers)
}

func (c *Container) resolve(serviceType reflect.Type) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, ErrScopeClosed
	}
	return c.resolveLocked(serviceType, nil)
}

func (c *Container) resolveLocked(serviceType reflect.Type, scope *Scope) (any, error) {
	def, ok := c.defs[serviceType]
	if !ok {
		return nil, errNotRegistered(serviceType)
	}

	switch def.lifetime {
	case Singleton:
		if instance, ok := c.singletons[serviceType]; ok {
			return instance, nil
		}
		instance, err := c.createLocked(def, scope)
		if err != nil {
			return nil, err
		}
		c.singletons[serviceType] = instance
		trackCloser(&c.disposers, instance)
		return instance, nil

	case Scoped:
		if scope == nil {
			return nil, fmt.Errorf("%w: %s", ErrScopeRequired, serviceType)
		}
		if instance, ok := scope.scoped[serviceType]; ok {
			return instance, nil
		}
		instance, err := c.createLocked(def, scope)
		if err != nil {
			return nil, err
		}
		scope.scoped[serviceType] = instance
		trackCloser(&scope.disposers, instance)
		return instance, nil

	case Transient:
		instance, err := c.createLocked(def, scope)
		if err != nil {
			return nil, err
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

func (c *Container) createLocked(def *definition, scope *Scope) (any, error) {
	serviceType := def.serviceType
	for _, t := range c.resolving {
		if t == serviceType {
			return nil, errCircular(c.resolving, serviceType)
		}
	}

	c.resolving = append(c.resolving, serviceType)
	defer func() {
		c.resolving = c.resolving[:len(c.resolving)-1]
	}()

	return def.create(&resolveContext{root: c, scope: scope})
}

func trackCloser(list *[]io.Closer, instance any) {
	if closer, ok := instance.(io.Closer); ok {
		*list = append(*list, closer)
	}
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
