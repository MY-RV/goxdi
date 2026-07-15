package container

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrNotRegistered is returned when resolving a type with no Add* registration.
	ErrNotRegistered = errors.New("service not registered")
	// ErrScopeRequired is returned when resolving a Scoped type without a Scope.
	ErrScopeRequired = errors.New("scoped service requires an active scope")
	// ErrClosed is returned when resolving or closing an already-closed Container or Scope.
	ErrClosed = errors.New("resolver is closed")
	// ErrScopeClosed is an alias of ErrClosed kept for compatibility; prefer ErrClosed.
	ErrScopeClosed = ErrClosed
	// ErrAlreadyRegistered is returned when Add* is called twice for the same T on one Builder.
	ErrAlreadyRegistered = errors.New("service already registered")
	// ErrCircularDependency is returned when a factory graph cycles during construction.
	ErrCircularDependency = errors.New("circular dependency detected")

	// errFactoryPanicked unblocks singleton singleflight waiters if a factory panics.
	errFactoryPanicked = errors.New("factory panicked during construction")
)

func errNotRegistered(t reflect.Type) error {
	return fmt.Errorf("%w: %s", ErrNotRegistered, t)
}

func errAlreadyRegistered(t reflect.Type) error {
	return fmt.Errorf("%w: %s", ErrAlreadyRegistered, t)
}

func errCircular(stack []reflect.Type, t reflect.Type) error {
	parts := make([]string, 0, len(stack)+1)
	for _, s := range stack {
		parts = append(parts, s.String())
	}
	parts = append(parts, t.String())
	return fmt.Errorf("%w: %v", ErrCircularDependency, parts)
}

func errNotAssignable(raw any, want reflect.Type) error {
	return fmt.Errorf("resolved %T is not assignable to %s", raw, want)
}
