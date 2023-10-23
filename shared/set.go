package shared

type Emptiness struct{}

type Set[T comparable] struct {
	values map[T]Emptiness
}

func NewSet[T comparable](values []T) *Set[T] {
	set := Set[T]{make(map[T]Emptiness)}
	for _, v := range values {
		set.values[v] = Emptiness{}
	}
	return &set
}

func NewEmptySet[T comparable]() *Set[T] {
	return NewSet([]T{})
}

func (set *Set[T]) Add(k T) {
	set.values[k] = Emptiness{}
}

func (set *Set[T]) Contains(k T) bool {
	_, exist := set.values[k]
	return exist
}

func (set *Set[T]) Size() int {
	return len(set.values)
}

func (set *Set[T]) Values() []T {
	result := []T{}
	for v, _ := range set.values {
		result = append(result, v)
	}
	return result
}
