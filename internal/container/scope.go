package container

import (
	"io"
	"reflect"
)

// Scope is a child lifetime. Scoped services are unique per Scope;
// Singletons are shared with the root Container.
//
// Do not share one Scope across goroutines. Open a Scope per unit of work.
type Scope struct {
	root      *Container
	scoped    map[reflect.Type]any
	disposers []io.Closer
	closed    bool
}

// NewScope creates a nested Scope that still shares Singletons with the root.
// The nested Scope starts with an empty Scoped cache (does not inherit parent Scoped instances).
func (s *Scope) NewScope() *Scope {
	return &Scope{
		root:   s.root,
		scoped: make(map[reflect.Type]any),
	}
}

// Close disposes Scoped/Transient instances created in this Scope that implement io.Closer.
// After Close, further resolves return ErrClosed.
func (s *Scope) Close() error {
	s.root.mu.Lock()
	defer s.root.mu.Unlock()

	if s.closed {
		return ErrClosed
	}
	s.closed = true
	return closeAll(s.disposers)
}

func (s *Scope) resolve(serviceType reflect.Type) (any, error) {
	s.root.mu.Lock()
	defer s.root.mu.Unlock()

	if s.closed {
		return nil, ErrClosed
	}
	if s.root.closed {
		return nil, ErrClosed
	}
	return s.root.resolveLocked(serviceType, s, nil)
}
