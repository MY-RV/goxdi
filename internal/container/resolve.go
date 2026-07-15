package container

import "reflect"

// Resolver is anything Get / MustGet can resolve from (Container, Scope, or
// the Resolver passed into a Factory). Prefer that factory Resolver for
// dependencies so circular detection and lifetime boundaries stay correct.
type Resolver interface {
	resolve(serviceType reflect.Type) (any, error)
}

// resolveContext is the Resolver passed into factories. stack is the
// construction chain used for ErrCircularDependency (per call graph, not global).
type resolveContext struct {
	root  *Container
	scope *Scope
	stack []reflect.Type
}

func (c *resolveContext) resolve(serviceType reflect.Type) (any, error) {
	c.root.mu.Lock()
	defer c.root.mu.Unlock()

	if c.root.closed {
		return nil, ErrClosed
	}
	if c.scope != nil && c.scope.closed {
		return nil, ErrClosed
	}
	return c.root.resolveLocked(serviceType, c.scope, c.stack)
}

// Get resolves T from a Container or Scope.
func Get[T any](from Resolver) (T, error) {
	var zero T
	raw, err := from.resolve(typeOf[T]())
	if err != nil {
		return zero, err
	}

	typed, ok := raw.(T)
	if !ok {
		return zero, errNotAssignable(raw, typeOf[T]())
	}
	return typed, nil
}

// MustGet resolves T or panics. Prefer inside factories for required deps.
func MustGet[T any](from Resolver) T {
	service, err := Get[T](from)
	if err != nil {
		panic(err)
	}
	return service
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
