package routerShared

const WILD_CARD_SYMBOL = "*"

type Path []string

type URISegment[T any] struct {
	Parent   *URISegment[T]
	Children map[string]*URISegment[T]
	Data     map[string]T
}

func (segment *URISegment[T]) Leaf() bool {
	return len(segment.Children) == 0
}

func (segment *URISegment[T]) Empty() bool {
	return len(segment.Data) == 0
}

func NewURISegment[T any](parent *URISegment[T]) *URISegment[T] {
	return &URISegment[T]{parent, make(map[string]*URISegment[T]), make(map[string]T)}
}

type URISegmentList[T any] []*URISegment[T]

func (segment *URISegment[T]) Match(path Path) URISegmentList[T] {
	if len(path) == 0 {
		return URISegmentList[T]{segment}
	}

	result := URISegmentList[T]{}
	if segment.Leaf() {
		return result
	}

	key := path[0]
	child, found := segment.Children[key]
	if found {
		subResult := child.Match(path[1:])
		result = append(result, subResult...)
	}

	child, found = segment.Children[WILD_CARD_SYMBOL]
	if found {
		subResult := child.Match(path[1:])
		result = append(result, subResult...)
	}

	return result
}

func (segment *URISegment[T]) Get(path Path) *URISegment[T] {
	if len(path) == 0 {
		return segment
	}

	key := path[0]

	child, found := segment.Children[key]
	if found {
		return child.Get(path[1:])
	}

	return nil
}

func (segment *URISegment[T]) GetSert(path Path) *URISegment[T] {
	if len(path) == 0 {
		return segment
	}

	key := path[0]

	child, found := segment.Children[key]
	if !found {
		child = NewURISegment(segment)
		segment.Children[key] = child
	}

	return child.GetSert(path[1:])
}

func (segment *URISegment[T]) PathDump() []Path {
	result := []Path{}
	if !segment.Empty() {
		result = append(result, Path{})
	}
	if segment.Leaf() {
		return result
	}
	for key, child := range segment.Children {
		childResult := child.PathDump()
		for _, childPath := range childResult {
			path := append(Path{key}, childPath...)
			result = append(result, path)
		}
	}
	return result
}
