# API reference

Package [`github.com/MY-RV/goxdi`](https://pkg.go.dev/github.com/MY-RV/goxdi).

This page lists the public surface. For behavior and guidance, use the guides linked below.

| Topic | Guide |
| --- | --- |
| Concepts | [Overview](../overview.md) |
| First program | [Getting started](../getting-started.md) |
| `Add*` | [Registering services](../guide/registering.md) |
| Lifetimes | [Lifetimes](../guide/lifetimes.md) |
| `Scope` / `Close` | [Scopes](../guide/scopes.md) |
| `Get` / `MustGet` | [Resolving](../guide/resolving.md) |
| Sentinels | [Errors](../guide/errors.md) |

## Types

| Name | Definition |
| --- | --- |
| `Lifetime` | `int` — retention policy for a registration |
| `Factory[T any]` | `func(Resolver) (T, error)` |
| `Resolver` | resolve target for `Get` / `MustGet` |
| `Builder` | mutable registrations before `Build` |
| `Container` | root container; owns singletons |
| `Scope` | child lifetime; owns scoped instances |

```go
const (
	Singleton Lifetime = iota
	Scoped
	Transient
)
```

`Lifetime.String()` returns `"Singleton"`, `"Scoped"`, `"Transient"`, or `"Unknown"`.

## Functions

```go
func NewBuilder() *Builder
func (b *Builder) Build() *Container

func AddSingleton[T any](b *Builder, factory Factory[T]) error
func AddScoped[T any](b *Builder, factory Factory[T]) error
func AddTransient[T any](b *Builder, factory Factory[T]) error

func (c *Container) NewScope() *Scope
func (c *Container) Close() error

func (s *Scope) NewScope() *Scope
func (s *Scope) Close() error

func Get[T any](from Resolver) (T, error)
func MustGet[T any](from Resolver) T
```

`Container` and `Scope` implement `Resolver`.

## Errors

```go
var (
	ErrNotRegistered      error
	ErrScopeRequired      error
	ErrClosed             error
	ErrScopeClosed        error // alias of ErrClosed
	ErrAlreadyRegistered  error
	ErrCircularDependency error
)
```
