package container

// Factory constructs a service T. Resolve dependencies with Get / MustGet.
type Factory[T any] func(r Resolver) (T, error)
