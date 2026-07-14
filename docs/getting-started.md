# Getting started

This guide gets you from zero to a resolving container. For concepts and vocabulary, see [Overview](overview.md).

## Prerequisites

- Go **1.23** or newer

## Install

```bash
go get github.com/MY-RV/goxdi
```

## Create a builder and register a service

A `Builder` holds registrations until you call `Build`. Each registration pairs a type `T` with a factory that returns `(T, error)`.

```go
package main

import (
	"fmt"

	"github.com/MY-RV/goxdi"
)

type Greeter struct {
	Msg string
}

func main() {
	b := goxdi.NewBuilder()

	if err := goxdi.AddSingleton(b, func(goxdi.Resolver) (*Greeter, error) {
		return &Greeter{Msg: "hello from goxdi"}, nil
	}); err != nil {
		panic(err)
	}

	root := b.Build()
	defer root.Close()

	g, err := goxdi.Get[*Greeter](root)
	if err != nil {
		panic(err)
	}
	fmt.Println(g.Msg)
}
```

What this does:

1. `NewBuilder` creates an empty registry.
2. `AddSingleton` provides `*Greeter` for the lifetime of the container.
3. `Build` freezes registrations into a root `Container`.
4. `Get[*Greeter]` resolves the instance (creating it on first use).
5. `Close` disposes tracked `io.Closer` singletons (none here).

## Wire a dependency

Factories receive a `Resolver`. Use it to ask for other services instead of constructing them inline.

```go
type DB struct{ Name string }

type Repo struct{ DB *DB }

b := goxdi.NewBuilder()

_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*DB, error) {
	return &DB{Name: "main"}, nil
})

_ = goxdi.AddScoped(b, func(r goxdi.Resolver) (*Repo, error) {
	db, err := goxdi.Get[*DB](r)
	if err != nil {
		return nil, err
	}
	return &Repo{DB: db}, nil
})

root := b.Build()
defer root.Close()

scope := root.NewScope()
defer scope.Close()

repo := goxdi.MustGet[*Repo](scope)
fmt.Println(repo.DB.Name)
```

IMPORTANT: `*Repo` is **scoped**. Resolving it from `root` returns `ErrScopeRequired`. Open a scope first.

## Composition checklist

Use this when wiring a real process:

1. Put registrations in one composition root (`main` or a dedicated `wire` package).
2. Register every type you will resolve, including **interface** tokens consumers depend on.
3. Call `Build()` once; keep the root for the process lifetime.
4. Call `NewScope()` once per unit of work (request, job, CLI command).
5. Prefer `Get` where errors should propagate; use `MustGet` inside factories you fully control.
6. Always `defer scope.Close()` and `defer root.Close()`.

## Next steps

You now have the happy path. Specialized guides cover the hard edges:

- [Registering services](guide/registering.md) — duplicates, interfaces, factory shape
- [Lifetimes](guide/lifetimes.md) — singleton vs scoped vs transient
- [Scopes](guide/scopes.md) — nesting and disposal
- [Resolving](guide/resolving.md) — `Get` vs `MustGet` and circular graphs
