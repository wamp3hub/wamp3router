package routerShared

import cmap "github.com/orcaman/concurrent-map/v2"

const WILD_CARD_SYMBOL = "*"

type Path []string

type URISegment[T any] struct {
	Parent   *URISegment[T]
	Children cmap.ConcurrentMap[string, *URISegment[T]]
	Data     cmap.ConcurrentMap[string, T]
}

func (segment *URISegment[T]) Leaf() bool {
	return segment.Children.IsEmpty()
}

func (segment *URISegment[T]) Empty() bool {
	return segment.Data.IsEmpty()
}

func NewURISegment[T any](parent *URISegment[T]) *URISegment[T] {
	return &URISegment[T]{parent, cmap.New[*URISegment[T]](), cmap.New[T]()}
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
	child, found := segment.Children.Get(key)
	if found {
		subResult := child.Match(path[1:])
		result = append(result, subResult...)
	}

	child, found = segment.Children.Get(WILD_CARD_SYMBOL)
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

	child, found := segment.Children.Get(key)
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

	child, found := segment.Children.Get(key)
	if !found {
		child = NewURISegment(segment)
		segment.Children.Set(key, child)
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
	for key, child := range segment.Children.Items() {
		childResult := child.PathDump()
		for _, childPath := range childResult {
			path := append(Path{key}, childPath...)
			result = append(result, path)
		}
	}
	return result
}
