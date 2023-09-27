package wamp3router

const WILD_CARD_SYMBOL = "*"

type URISegment[T any] struct {
	Parent   *URISegment[T]
	Children map[string]*URISegment[T]
	Data     map[string]T
}

type URISegmentList[T any] []*URISegment[T]

func (segment *URISegment[T]) Get(path []string) URISegmentList[T] {
	if len(path) == 0 {
		return URISegmentList[T]{segment}
	}

	result := URISegmentList[T]{}

	key := path[0]

	child, exist := segment.Children[key]
	if exist {
		sub := child.Get(path[1:])
		result = append(result, sub...)
	}

	child, exist = segment.Children[WILD_CARD_SYMBOL]
	if exist {
		sub := child.Get(path[1:])
		result = append(result, sub...)
	}

	return result
}

func (segment *URISegment[T]) GetSert(path []string) *URISegment[T] {
	if len(path) == 0 {
		return segment
	}

	key := path[0]

	child, exist := segment.Children[key]
	if !exist {
		child = &URISegment[T]{
			Parent:   segment,
			Children: make(map[string]*URISegment[T]),
			Data:     make(map[string]T),
		}
		segment.Children[key] = child
	}

	return child.GetSert(path[1:])
}
