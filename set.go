package wamp3router

type Set[T comparable] struct {
	values map[T]struct{}
}

func NewSet[T comparable](values []T) *Set[T] {
	instance := Set[T]{values: make(map[T]struct{})}
	for _, v := range values {
		instance.values[v] = struct{}{}
	}
	return &instance
}

func (instance *Set[T]) Add(k T) {
	instance.values[k] = struct{}{}
}

func (instance *Set[T]) Contains(k T) bool {
	_, exist := instance.values[k]
	return exist
}
