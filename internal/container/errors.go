package container

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrNotRegistered      = errors.New("service not registered")
	ErrScopeRequired      = errors.New("scoped service requires an active scope")
	ErrScopeClosed        = errors.New("scope is closed")
	ErrAlreadyRegistered  = errors.New("service already registered")
	ErrCircularDependency = errors.New("circular dependency detected")
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
