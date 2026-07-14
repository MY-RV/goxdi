# goxdi

Typed dependency injection for Go.

```bash
go get github.com/MY-RV/goxdi
```

```go
import "github.com/MY-RV/goxdi"

b := goxdi.NewBuilder()
_ = goxdi.AddSingleton(b, func(goxdi.Resolver) (*DB, error) {
	return &DB{}, nil
})
root := b.Build()
defer root.Close()

scope := root.NewScope()
defer scope.Close()

db := goxdi.MustGet[*DB](scope)
_ = db
```

## Docs

Full guides: [`docs/`](docs/README.md)

- [Overview](docs/overview.md)
- [Getting started](docs/getting-started.md)
- [Lifetimes](docs/guide/lifetimes.md)
- [API reference](docs/reference/api.md)

## License

MIT
