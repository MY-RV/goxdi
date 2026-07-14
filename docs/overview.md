# Overview

goxdi is a system for organizing and sharing dependencies across a Go application by **supplying** them from outside a type, instead of constructing them inside it.

As an application grows, you need to reuse clients, repositories, and request-bound state without hard-wiring constructors everywhere. Dependency injection (DI) addresses that by separating **registration** (what exists) from **resolution** (who asks for it).

TIP: If you want a runnable path first, see [Getting started](getting-started.md). This page defines vocabulary used everywhere else.

## What problems DI solves

- **Maintainability** — construction lives in one composition root; business code depends on types, not `new` graphs.
- **Testability** — register fakes or stubs for the same type token without changing consumers.
- **Lifetime control** — decide once whether an instance is process-wide, per-request, or always fresh.

## Provide vs resolve

You interact with goxdi in two ways:

1. **Provide** — make a value available by registering a `Factory[T]` on a `Builder`.
2. **Resolve** — ask for `T` with `Get[T]` or `MustGet[T]` from a `Container` or `Scope`.

A *dependency* is any value a type needs but does not create itself: a `*sql.DB`, a repository interface, a logger, configuration, etc.

```text
provide                          resolve
───────                          ───────
Builder.Add*(factory)     →      Get[T] / MustGet[T]
      │                                ▲
      └──── Build() → Container ───────┘
                        │
                   NewScope() → Scope ─ Get[T]
```

## Core concepts

| Concept | Role |
| --- | --- |
| `Builder` | Mutable registry of factories before the container exists |
| `Container` | Root resolver; owns **singleton** instances |
| `Scope` | Child resolver; owns **scoped** instances for one unit of work |
| `Factory[T]` | `func(Resolver) (T, error)` — how to create `T` |
| `Resolver` | Anything you can resolve from (`Container`, `Scope`, or the context passed into a factory) |
| `Lifetime` | How long a created instance is retained: singleton, scoped, or transient |

## How a request typically flows

1. At process start, register services on a `Builder`, then `Build()` a root `Container`.
2. For each HTTP request, job, or command, open a `Scope`.
3. Resolve the entry service (`Handler`, `UseCase`, …). Its factory resolves further dependencies through the same `Resolver`.
4. Close the scope (and later the root) so resources that implement `io.Closer` are disposed.

## What goxdi is not

goxdi is intentionally small:

- No decorator magic, no `Inject` struct tags, no auto-wiring by constructor reflection.
- No named/keyed multi-bind for the same type in core.
- No framework lifecycle hooks beyond `Close` + `io.Closer`.

Extensions belong in your app or a separate sugar module that composes `Factory` and `Add*`.

## Next steps

- [Getting started](getting-started.md) — install and a first program
- [Registering services](guide/registering.md) — how provide works in detail
- [Lifetimes](guide/lifetimes.md) — when instances are shared or recreated
