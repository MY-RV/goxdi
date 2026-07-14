# Registering services

Registration is how you **provide** values to goxdi. You associate a type token `T` with a factory that knows how to create it.

TIP: Registration happens only on a `Builder`, before `Build()`. After `Build()`, the set of services is fixed for that container.

## The factory contract

```go
type Factory[T any] func(r goxdi.Resolver) (T, error)
```

- `r` is the resolver for **this construction**. Use it to pull dependencies with `Get` / `MustGet`.
- Return a concrete `T` (or an interface value if `T` is an interface).
- Return a non-nil `error` to fail construction; `Get` surfaces that error to the caller.

```go
b := goxdi.NewBuilder()

err := goxdi.AddSingleton(b, func(r goxdi.Resolver) (*Logger, error) {
	cfg, err := goxdi.Get[*Config](r)
	if err != nil {
		return nil, err
	}
	return NewLogger(cfg.LogLevel), nil
})
```

## Registration APIs

| Function | Lifetime applied |
| --- | --- |
| `AddSingleton[T](b, factory)` | One instance per root container |
| `AddScoped[T](b, factory)` | One instance per scope |
| `AddTransient[T](b, factory)` | New instance on every resolve |

Choose the lifetime for **how the instance should be shared**, not for how expensive construction is alone. Details: [Lifetimes](lifetimes.md).

## Type tokens

goxdi keys registrations by the reflected type of `T`:

- `AddSingleton[*sql.DB](...)` and `AddSingleton[Store](...)` are different tokens.
- Consumers must resolve the **same** `T` you registered.

### Registering interfaces

Prefer depending on interfaces in application code. Register the interface as `T` and return a concrete implementation:

```go
type Store interface {
	Get(id string) (Item, error)
}

type memoryStore struct{}

func (memoryStore) Get(id string) (Item, error) { /* ... */ }

_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (Store, error) {
	return memoryStore{}, nil
})

// later
store, err := goxdi.Get[Store](root)
```

Avoid registering only `*memoryStore` if callers need `Store` — they would have to know the concrete type.

## One registration per type

Each `T` may be registered once on a given builder.

```go
_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*DB, error) {
	return &DB{Name: "first"}, nil
})

err := goxdi.AddSingleton(b, func(goxdi.Resolver) (*DB, error) {
	return &DB{Name: "second"}, nil
})
// err is ErrAlreadyRegistered — "first" remains registered
```

IMPORTANT: A failed duplicate `Add*` does **not** replace the original factory.

There is no built-in “override for tests” API. For tests, build a **separate** `Builder` with the fakes you need.

## What to put in a factory

**Prefer** — create and wire the instance:

```go
_ = goxdi.AddScoped(b, func(r goxdi.Resolver) (*Repo, error) {
	return &Repo{DB: goxdi.MustGet[*sql.DB](r)}, nil
})
```

**Avoid** — hiding process-wide side effects that belong in `main` (unless that is intentional):

```go
// Avoid: factory mutates global mutable state as a surprise
_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*Client, error) {
	http.DefaultClient.Timeout = time.Second // surprising
	return NewClient(), nil
})
```

**Avoid** — capturing an outer `Scope` or `Container` in the closure when `r` is available. That bypasses the resolve context goxdi passes during construction (and can break nested resolution / circular detection expectations).

```go
// Avoid
scope := root.NewScope()
_ = goxdi.AddScoped(b, func(goxdi.Resolver) (*Repo, error) {
	return &Repo{DB: goxdi.MustGet[*sql.DB](scope)}, nil // captured scope
})
```

```go
// Prefer
_ = goxdi.AddScoped(b, func(r goxdi.Resolver) (*Repo, error) {
	return &Repo{DB: goxdi.MustGet[*sql.DB](r)}, nil
})
```

NOTE: You cannot call `Add*` after `Build()` on that container. Registration is a compile-time-of-the-graph step, not a runtime reconfiguration API.

## Building

```go
root := b.Build()
defer root.Close()
```

`Build` copies definitions into a new `Container`. The builder is not required afterward. Missing dependencies are **not** validated at build time — they surface on first `Get`. See [Resolving](resolving.md).

## Next steps

- [Lifetimes](lifetimes.md) — singleton, scoped, transient semantics
- [Resolving](resolving.md) — how factories pull dependencies
- [Errors](errors.md) — `ErrAlreadyRegistered` and friends
