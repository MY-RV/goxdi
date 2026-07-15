package container

// Factory constructs a service T.
// Resolve dependencies with Get / MustGet on r (the factory Resolver), not a
// captured Container or Scope, so circular detection and lifetimes stay correct.
type Factory[T any] func(r Resolver) (T, error)
