package container

// Lifetime controls how long a resolved instance is retained.
type Lifetime int

const (
	Singleton Lifetime = iota
	Scoped
	Transient
)

func (l Lifetime) String() string {
	switch l {
	case Singleton:
		return "Singleton"
	case Scoped:
		return "Scoped"
	case Transient:
		return "Transient"
	default:
		return "Unknown"
	}
}
