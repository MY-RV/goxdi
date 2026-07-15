# Resolving

Resolution is how you **ask** goxdi for a value of type `T`. The entry points are `Get` and `MustGet`.

```go
func Get[T any](from Resolver) (T, error)
func MustGet[T any](from Resolver) T
```

Anything that implements `Resolver` can be passed as `from`: the root `Container`, a `Scope`, or the resolver handed to a factory while it runs.

## `Get` vs `MustGet`

### `Get`

Returns `(T, error)`. Use it at boundaries where failure should become a returned error (HTTP handlers, job runners, CLI).

```go
repo, err := goxdi.Get[*Repo](scope)
if err != nil {
	return err
}
```

### `MustGet`

Calls `Get` and panics if `err != nil`. Use it when a missing dependency is a programmer error and the process should not continue half-initialized — typically inside factories at the composition root.

```go
_ = goxdi.AddScoped(b, func(r goxdi.Resolver) (*Repo, error) {
	return &Repo{DB: goxdi.MustGet[*sql.DB](r)}, nil
})
```

There is no `TryGet`. Optional dependencies are an application pattern: either register a no-op implementation, or call `Get` and branch on `ErrNotRegistered`.

## The resolve context inside factories

When goxdi invokes a factory, it passes a `Resolver` tied to the **current** root + optional scope. The container mutex is released while the factory runs so nested `Get` calls do not deadlock. Dependencies resolved through that `r` participate correctly in:

- the active lifetime caches
- circular-dependency detection
- disposer tracking for the creating boundary

**Prefer** resolving through `r`:

```go
func(r goxdi.Resolver) (*OrderService, error) {
	repo, err := goxdi.Get[*OrderRepo](r)
	if err != nil {
		return nil, err
	}
	payments, err := goxdi.Get[Payments](r)
	if err != nil {
		return nil, err
	}
	return &OrderService{Repo: repo, Payments: payments}, nil
}
```

**Avoid** resolving through a captured outer variable when constructing a graph that should use the active context:

```go
// Avoid — skips the factory resolve context
func(r goxdi.Resolver) (*OrderService, error) {
	return &OrderService{Repo: goxdi.MustGet[*OrderRepo](outerScope)}, nil
}
```

Prefer `r`. Captured `Container`/`Scope` values do not carry the construction stack, so circular dependencies resolved that way may hang instead of returning `ErrCircularDependency`.

## Dependency graphs

Factories may resolve other registered types. goxdi creates dependencies on demand (lazy), caching according to each type’s lifetime.

```go
_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*DB, error) { return &DB{}, nil })
_ = goxdi.AddScoped(b, func(r goxdi.Resolver) (*Repo, error) {
	return &Repo{DB: goxdi.MustGet[*DB](r)}, nil
})
_ = goxdi.AddTransient(b, func(r goxdi.Resolver) (*Service, error) {
	return &Service{Repo: goxdi.MustGet[*Repo](r)}, nil
})

scope := root.NewScope()
defer scope.Close()

s1 := goxdi.MustGet[*Service](scope)
s2 := goxdi.MustGet[*Service](scope)
// s1 != s2              (transient)
// s1.Repo == s2.Repo    (scoped, same scope)
// s1.Repo.DB == s2.Repo.DB  (singleton)
```

### Validation timing

`Build()` does **not** walk factories to prove the graph is complete. An unregistered dependency fails at the first `Get` that needs it (`ErrNotRegistered`).

HELPFUL: At process boot, resolve your composition root once inside a throwaway scope to fail fast before serving traffic.

## Circular dependencies

If A’s factory resolves B while B’s factory resolves A, goxdi returns `ErrCircularDependency` with a path of types.

```go
// A → B → A
_, err := goxdi.Get[*A](root)
// errors.Is(err, goxdi.ErrCircularDependency)
```

goxdi does not invent lazy proxies to break cycles. Fix the design:

- introduce an interface to invert one edge
- move shared state to a third type
- pass a callback/event instead of a hard constructor dependency

## Closed resolvers

Resolving from a closed scope or closed root fails with `ErrClosed`. Create a new scope after close; do not reuse a closed one.

## Next steps

- [Errors](errors.md) — mapping failures to sentinels
- [Lifetimes](lifetimes.md) — why two resolves may or may not share instances
- [Scopes](scopes.md) — which resolver to pass (`root` vs `scope`)
