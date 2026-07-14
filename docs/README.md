# Documentation

goxdi is a typed dependency-injection library for Go. You **provide** services by registering factories, then **resolve** them later with `Get` / `MustGet`.

## Start here

| Guide | What you learn |
| --- | --- |
| [Overview](overview.md) | What goxdi is, provide vs resolve, core vocabulary |
| [Getting started](getting-started.md) | Install, first working program, composition checklist |

## In-depth guides

Each page focuses on **one mechanism**, with behavior, examples, and pitfalls.

| Guide | What you learn |
| --- | --- |
| [Registering services](guide/registering.md) | `Add*`, factories, duplicates, interfaces as tokens |
| [Lifetimes](guide/lifetimes.md) | Singleton, scoped, transient — when instances are shared |
| [Scopes](guide/scopes.md) | `NewScope`, nesting, `Close`, `io.Closer` disposal |
| [Resolving](guide/resolving.md) | `Get` / `MustGet`, resolve context, dependency graphs |
| [Errors](guide/errors.md) | Sentinel errors and how to handle them |

## Reference

| Page | What you learn |
| --- | --- |
| [API reference](reference/api.md) | Public types, functions, and error variables |
