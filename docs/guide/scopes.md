# Scopes

A **scope** is a child resolution context under a root `Container`. It shares singletons with the root and owns its own cache of scoped (and tracked transient) instances.

Use scopes to model a **unit of work**: an HTTP request, a background job, a CLI command, a message handler.

TIP: Keep one root container for the process. Open and close many scopes over its lifetime.

## Creating a scope

```go
root := b.Build()
defer root.Close()

scope := root.NewScope()
defer scope.Close()

svc := goxdi.MustGet[*RequestService](scope)
```

`NewScope` is cheap: it allocates an empty scoped map and points at the same root.

## What a scope shares vs owns

| Kind | Where it lives | Visible from |
| --- | --- | --- |
| Singleton | Root | Root and every scope |
| Scoped | That scope’s map | That scope only |
| Transient | Not cached | Created on each resolve; closer tracked on creator |

```text
Container (root)
├── singletons:  *sql.DB, *Logger, …
├── Scope A
│   └── scoped:  *UnitOfWork, …
└── Scope B
    └── scoped:  *UnitOfWork, …   // different instances than A
```

## Nested scopes

Scopes can nest:

```go
parent := root.NewScope()
defer parent.Close()

child := parent.NewScope()
defer child.Close()
```

Properties:

- Child still resolves **singletons** from the same root.
- Child has an **empty** scoped cache of its own — it does not inherit parent scoped instances.
- Closing the child does not close the parent; close both if you opened both.

IMPORTANT: Nested scopes are for nested units of work (e.g. an inner batch inside a job). They are not a substitute for overriding registrations (goxdi has no hierarchical provider shadowing like Angular’s element injectors).

## Closing a scope

```go
if err := scope.Close(); err != nil {
	// handle disposer errors
}
```

After `Close`:

- Further resolves on that scope return `ErrScopeClosed`.
- Calling `Close` again returns `ErrScopeClosed`.

Always pair `NewScope` with `defer Close()` at the same layer that opened it.

## Disposal with `io.Closer`

When a factory returns a value that implements `io.Closer`, goxdi appends it to a disposer list for the owning boundary:

| Instance lifetime | Disposer list | Closed when |
| --- | --- | --- |
| Singleton | Root | `Container.Close` |
| Scoped | Scope | `Scope.Close` |
| Transient | Scope that created it, or root if resolved on root | matching `Close` |

Closers run in **reverse creation order** (LIFO), so dependents can close before dependencies when construction order was dependency-first.

```go
type closerDB struct{ closed *bool }

func (c *closerDB) Close() error {
	*c.closed = true
	return nil
}

closed := false
_ = goxdi.AddScoped(b, func(goxdi.Resolver) (*closerDB, error) {
	return &closerDB{closed: &closed}, nil
})

root := b.Build()
defer root.Close()

scope := root.NewScope()
_ = goxdi.MustGet[*closerDB](scope)
_ = scope.Close()
// closed == true
```

NOTE: Only values that implement `io.Closer` at the concrete resolved type are tracked. Wrapping resources without exposing `Close` on the registered type means goxdi will not dispose them.

### Prefer / Avoid

**Prefer** — implement `Close` on types that own OS resources, and rely on scope/root close:

```go
type ConnPool struct{ /* ... */ }

func (p *ConnPool) Close() error { return p.pool.Close() }

_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*ConnPool, error) {
	return OpenPool(dsn)
})
```

**Avoid** — resolving scoped/`Closer` types and discarding the scope without `Close` (resource leak).

**Avoid** — closing the root while scopes are still in use. Resolve/close ordering should be: finish scopes → close root.

## Root as a resolver

The root implements `Resolver` too. That is appropriate for singletons and for transients that do not need a request boundary.

```go
db := goxdi.MustGet[*sql.DB](root) // fine for singleton
```

Scoped types must still go through a scope.

## Concurrency note

Resolution under a container is serialized with a root mutex (scopes lock the same root mutex). Concurrent units of work should use **separate scopes**, not share one mutable scope across goroutines unless you externalize synchronization.

## Next steps

- [Lifetimes](lifetimes.md) — which lifetime belongs on which type
- [Resolving](resolving.md) — resolving from root vs scope inside factories
- [Errors](errors.md) — `ErrScopeRequired`, `ErrScopeClosed`
