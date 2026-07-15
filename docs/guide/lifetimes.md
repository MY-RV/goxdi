# Lifetimes

A **lifetime** controls how long a resolved instance is retained and which cache stores it.

goxdi defines three lifetimes:

| Lifetime | Cache | Identity |
| --- | --- | --- |
| `Singleton` | Root `Container` | One instance for the container |
| `Scoped` | Current `Scope` | One instance per scope |
| `Transient` | None | New instance on every resolve |

You pick a lifetime when you register (`AddSingleton`, `AddScoped`, `AddTransient`). You do not pass a lifetime flag at resolve time.

TIP: Think in terms of *sharing boundaries* (process vs request vs call), not only “cheap vs expensive to construct”.

## Singleton

A singleton is created at most once per root container and reused for every subsequent resolve of that type — from the root **or** from any scope under that root.

```go
_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
})

root := b.Build()
defer root.Close()

s1 := root.NewScope()
defer s1.Close()
s2 := root.NewScope()
defer s2.Close()

a := goxdi.MustGet[*sql.DB](s1)
b := goxdi.MustGet[*sql.DB](s2)
c := goxdi.MustGet[*sql.DB](root)
// a == b == c
```

### Good fits

- Database pools, HTTP clients, gRPC clients
- Process configuration and feature flags loaded at startup
- Loggers and metrics reporters shared by the process

### Disposal

If the instance implements `io.Closer`, goxdi tracks it on the **root** and closes it during `Container.Close`, in reverse creation order. See [Scopes](scopes.md#disposal-with-iocloser).

### Singleton factories and scoped dependencies

Singleton construction always runs **without** an ambient scope. Resolving a scoped type from inside a singleton factory returns `ErrScopeRequired`. That prevents a process-wide instance from capturing a unit-of-work dependency.

## Scoped

A scoped instance is unique to one `Scope`. Resolving the same type again in that scope returns the same value. A sibling or nested scope creates its own.

```go
_ = goxdi.AddScoped(b, func(r goxdi.Resolver) (*UnitOfWork, error) {
	return NewUnitOfWork(goxdi.MustGet[*sql.DB](r)), nil
})

root := b.Build()
defer root.Close()

s1 := root.NewScope()
defer s1.Close()
s2 := root.NewScope()
defer s2.Close()

u1a := goxdi.MustGet[*UnitOfWork](s1)
u1b := goxdi.MustGet[*UnitOfWork](s1)
u2 := goxdi.MustGet[*UnitOfWork](s2)
// u1a == u1b
// u1a != u2
```

### Scoped services require a scope

Resolving a scoped type from the root container fails:

```go
_, err := goxdi.Get[*UnitOfWork](root)
// errors.Is(err, goxdi.ErrScopeRequired)
```

That is intentional: scoped types encode “this needs a unit-of-work boundary”.

### Good fits

- Unit of work / per-request DbContext analogues
- Request-correlated services (authorization context holders you build inside the scope)
- Caches that must not leak across requests

## Transient

A transient factory runs on **every** resolve. Instances are never reused by the container.

```go
_ = goxdi.AddTransient(b, func(r goxdi.Resolver) (*Handler, error) {
	return &Handler{Repo: goxdi.MustGet[*Repo](r)}, nil
})

scope := root.NewScope()
defer scope.Close()

h1 := goxdi.MustGet[*Handler](scope)
h2 := goxdi.MustGet[*Handler](scope)
// h1 != h2
```

### Good fits

- Lightweight helpers that must not retain accidental shared state
- Objects that are unsafe to reuse across calls

### Disposal

If a transient implements `io.Closer`, goxdi still tracks it on the **creating** scope (or the root if resolved without a scope) and closes it when that scope/root closes. Prefer not to make long-lived resources transient.

## Choosing a lifetime

| Question | Prefer |
| --- | --- |
| Must every caller see the same instance? | Singleton |
| Must isolation match a request/job? | Scoped |
| Must two resolves never share state? | Transient |
| Holds sockets, files, or pools? | Singleton (sometimes scoped) + `io.Closer` |

### Prefer / Avoid

**Prefer** singleton for shared infrastructure:

```go
_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
})
```

**Avoid** scoped database pools (easy to create many pools per process):

```go
// Avoid unless you truly need a pool per scope
_ = goxdi.AddScoped(b, func(goxdi.Resolver) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
})
```

**Prefer** scoped for request workflow state:

```go
_ = goxdi.AddScoped(b, func(r goxdi.Resolver) (*UnitOfWork, error) {
	return NewUnitOfWork(goxdi.MustGet[*sql.DB](r)), nil
})
```

**Avoid** singleton for types that accumulate per-request mutable state (race + leaks across users).

## Interaction with nested scopes

Nested scopes (`scope.NewScope()`) do **not** create nested singleton caches. Singletons always live on the root. Only the scoped cache is per-scope. Details and diagrams: [Scopes](scopes.md).

## Next steps

- [Scopes](scopes.md) — opening, nesting, and closing lifetimes
- [Resolving](resolving.md) — how lifetimes interact with dependency graphs
- [Registering services](registering.md) — attaching a lifetime at provide time
