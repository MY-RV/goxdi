# goxdi

Typed dependency injection for Go.

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

## License

MIT
