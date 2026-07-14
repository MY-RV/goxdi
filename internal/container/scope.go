package container

import (
	"io"
	"reflect"
)

// Scope is a child lifetime. Scoped services are unique per scope;
// singletons are shared with the root Container.
type Scope struct {
	root      *Container
	scoped    map[reflect.Type]any
	disposers []io.Closer
	closed    bool
}

// NewScope creates a nested scope that still shares singletons with the root.
func (s *Scope) NewScope() *Scope {
	return &Scope{
		root:   s.root,
		scoped: make(map[reflect.Type]any),
	}
}

// Close disposes scoped/transient instances created in this scope that implement io.Closer.
func (s *Scope) Close() error {
	s.root.mu.Lock()
	defer s.root.mu.Unlock()

	if s.closed {
		return ErrScopeClosed
	}
	s.closed = true
	return closeAll(s.disposers)
}

func (s *Scope) resolve(serviceType reflect.Type) (any, error) {
	s.root.mu.Lock()
	defer s.root.mu.Unlock()

	if s.closed {
		return nil, ErrScopeClosed
	}
	if s.root.closed {
		return nil, ErrScopeClosed
	}
	return s.root.resolveLocked(serviceType, s)
}
