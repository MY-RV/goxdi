// Package goxdi is a typed dependency-injection library for Go.
//
// This root package is the public API. Implementation lives in internal/container.
package goxdi

import "github.com/MY-RV/goxdi/internal/container"

type (
	Lifetime       = container.Lifetime
	Factory[T any] = container.Factory[T]
	Resolver       = container.Resolver
	Builder        = container.Builder
	Container      = container.Container
	Scope          = container.Scope
)

const (
	Singleton = container.Singleton
	Scoped    = container.Scoped
	Transient = container.Transient
)

var (
	ErrNotRegistered      = container.ErrNotRegistered
	ErrScopeRequired      = container.ErrScopeRequired
	ErrScopeClosed        = container.ErrScopeClosed
	ErrAlreadyRegistered  = container.ErrAlreadyRegistered
	ErrCircularDependency = container.ErrCircularDependency
)

func NewBuilder() *Builder {
	return container.NewBuilder()
}

func AddSingleton[T any](b *Builder, factory Factory[T]) error {
	return container.AddSingleton(b, factory)
}

func AddScoped[T any](b *Builder, factory Factory[T]) error {
	return container.AddScoped(b, factory)
}

func AddTransient[T any](b *Builder, factory Factory[T]) error {
	return container.AddTransient(b, factory)
}

func Get[T any](from Resolver) (T, error) {
	return container.Get[T](from)
}

func MustGet[T any](from Resolver) T {
	return container.MustGet[T](from)
}
