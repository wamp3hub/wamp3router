package wamp3router

type Emptiness struct{}

type Set[T comparable] struct {
	values map[T]Emptiness
}

func NewSet[T comparable](values []T) *Set[T] {
	instance := Set[T]{make(map[T]Emptiness)}
	for _, v := range values {
		instance.values[v] = Emptiness{}
	}
	return &instance
}

func NewEmptySet[T comparable]() *Set[T] {
	return NewSet([]T{})
}

func (instance *Set[T]) Add(k T) {
	instance.values[k] = Emptiness{}
}

func (instance *Set[T]) Contains(k T) bool {
	_, exist := instance.values[k]
	return exist
}

func (instance *Set[T]) Size() int {
	return len(instance.values)
}
