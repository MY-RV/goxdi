package container

import "reflect"

// Builder registers services before Build.
type Builder struct {
	defs map[reflect.Type]*definition
}

// NewBuilder creates an empty builder.
func NewBuilder() *Builder {
	return &Builder{
		defs: make(map[reflect.Type]*definition),
	}
}

// Build returns the root container.
func (b *Builder) Build() *Container {
	defs := make(map[reflect.Type]*definition, len(b.defs))
	for k, v := range b.defs {
		defs[k] = v
	}
	return newContainer(defs)
}

// AddSingleton registers a singleton factory.
func AddSingleton[T any](b *Builder, factory Factory[T]) error {
	return add(b, Singleton, factory)
}

// AddScoped registers a scoped factory.
func AddScoped[T any](b *Builder, factory Factory[T]) error {
	return add(b, Scoped, factory)
}

// AddTransient registers a transient factory.
func AddTransient[T any](b *Builder, factory Factory[T]) error {
	return add(b, Transient, factory)
}

func add[T any](b *Builder, lifetime Lifetime, factory Factory[T]) error {
	serviceType := typeOf[T]()
	if _, exists := b.defs[serviceType]; exists {
		return errAlreadyRegistered(serviceType)
	}

	b.defs[serviceType] = &definition{
		lifetime:    lifetime,
		serviceType: serviceType,
		create: func(r Resolver) (any, error) {
			return factory(r)
		},
	}
	return nil
}
