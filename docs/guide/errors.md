# Errors

goxdi uses sentinel errors. Branch with `errors.Is` — do not compare error strings.

```go
import "errors"

_, err := goxdi.Get[*Repo](scope)
if errors.Is(err, goxdi.ErrNotRegistered) {
	// …
}
```

## Sentinel catalog

| Sentinel | Typical cause |
| --- | --- |
| `ErrNotRegistered` | Resolving a type that was never `Add*`-ed |
| `ErrAlreadyRegistered` | Second `Add*` for the same `T` on one builder |
| `ErrScopeRequired` | Resolving a **scoped** type from the root container |
| `ErrScopeClosed` | `Get` / `Close` after the scope or root was closed |
| `ErrCircularDependency` | Factory cycle detected during creation |

## `ErrNotRegistered`

Surfaces at resolve time, including when a factory asks for a missing dependency.

```go
_ = goxdi.AddSingleton(b, func(r goxdi.Resolver) (*Repo, error) {
	db, err := goxdi.Get[*DB](r) // not registered
	if err != nil {
		return nil, err
	}
	return &Repo{DB: db}, nil
})

_, err := goxdi.Get[*Repo](root)
// errors.Is(err, goxdi.ErrNotRegistered)
```

**Prefer** failing boot by resolving the app entrypoint once before serving.

**Avoid** treating “not registered” as a normal control-flow signal in hot paths unless optional deps are a deliberate design.

## `ErrAlreadyRegistered`

Returned by `Add*` when `T` already exists. The original registration is unchanged.

```go
err := goxdi.AddSingleton(b, secondFactory)
if errors.Is(err, goxdi.ErrAlreadyRegistered) {
	// keep first factory; do not assume override
}
```

## `ErrScopeRequired`

Means “this type needs a unit-of-work boundary”.

```go
_, err := goxdi.Get[*UnitOfWork](root)
if errors.Is(err, goxdi.ErrScopeRequired) {
	scope := root.NewScope()
	defer scope.Close()
	uow, err := goxdi.Get[*UnitOfWork](scope)
	_ = uow
	_ = err
}
```

## `ErrScopeClosed`

Returned when using a closed scope/root, including a second `Close`.

**Prefer**:

```go
scope := root.NewScope()
defer scope.Close()
// resolve only while scope is open
```

**Avoid** storing a scope on a long-lived struct and resolving after the request finished.

## `ErrCircularDependency`

The error wraps the sentinel and includes a type path. Break the cycle in your model; see [Resolving](resolving.md#circular-dependencies).

## Factory errors

If a factory returns a custom error, `Get` returns it as-is (it is not rewritten into a sentinel).

```go
_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*DB, error) {
	return nil, fmt.Errorf("open db: %w", errOpen)
})
```

You may wrap sentinels yourself if you need more context:

```go
db, err := goxdi.Get[*DB](r)
if err != nil {
	return nil, fmt.Errorf("repo: %w", err)
}
```

## `MustGet` and panics

`MustGet` panics with the same `error` value `Get` would return. That panic is recoverable with `recover`, but recovering DI failures usually hides misconfiguration — prefer `Get` at recoverable boundaries.

## Next steps

- [API reference](../reference/api.md) — error variable declarations
- [Resolving](resolving.md) — where errors arise in the graph
- [Registering services](registering.md) — duplicate registration behavior
