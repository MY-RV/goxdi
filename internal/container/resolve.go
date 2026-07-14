package container

import "reflect"

// Resolver resolves services. Implemented by Container and Scope.
// Prefer Get / MustGet over calling resolve directly.
type Resolver interface {
	resolve(serviceType reflect.Type) (any, error)
}

// resolveContext is used while creating a service (lock already held).
type resolveContext struct {
	root  *Container
	scope *Scope
}

func (c *resolveContext) resolve(serviceType reflect.Type) (any, error) {
	return c.root.resolveLocked(serviceType, c.scope)
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

// MustGet resolves T or panics.
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
