package wamp3router

import (
	"errors"
	"log"
	"regexp"
	"strings"

	client "wamp3go"
)

var URI_RE, _ = regexp.Compile(`^(\*{1,2}|[_0-9a-z]+)(\.(\*{1,2}|[_0-9a-z]+))*$`)

func parseURI(v string) ([]string, error) {
	if !URI_RE.MatchString(v) {
		return nil, errors.New("InvalidURI")
	}

	result := strings.Split(v, ".")
	return result, nil
}

type ResourceList[T any] []*client.Resource[T]

type URIM[T any] struct {
	root    *URISegment[*client.Resource[T]]
	storage Storage
}

func NewURIM[T any](storage Storage) *URIM[T] {
	return &URIM[T]{
		root: &URISegment[*client.Resource[T]]{
			Parent:   nil,
			Children: make(map[string]*URISegment[*client.Resource[T]]),
			Data:     make(map[string]*client.Resource[T]),
		},
		storage: storage,
	}
}

// returns empty slice if something went wrong
func (urim *URIM[T]) Match(uri string) ResourceList[T] {
	resourceList := ResourceList[T]{}

	uriSegmentList, e := parseURI(uri)
	if e == nil {
		for _, segment := range urim.root.Get(uriSegmentList) {
			for _, resource := range segment.Data {
				resourceList = append(resourceList, resource)
			}
		}
	}

	return resourceList
}

func (urim *URIM[T]) Add(resource *client.Resource[T]) error {
	uriSegmentList, e := parseURI(resource.URI)
	if e != nil {
		return e
	}

	resourceList := ResourceList[T]{}
	urim.storage.Get("resources", resource.AuthorID, &resourceList)

	resourceList = append(resourceList, resource)
	e = urim.storage.Set("resources", resource.AuthorID, resourceList)
	if e != nil {
		return e
	}

	segment := urim.root.GetSert(uriSegmentList)
	segment.Data[resource.ID] = resource

	return nil
}

// returns empty list if something went wrong
func (urim *URIM[T]) GetByAuthor(ID string) ResourceList[T] {
	resourceList := ResourceList[T]{}

	e := urim.storage.Get("resources", ID, &resourceList)
	if e != nil {
		log.Printf("Matcher.GetByAuthor %s", e)
	}

	return resourceList
}

// pass empty routeID to remove all resources
func (urim *URIM[T]) DeleteByAuthor(ID string, routeID string) error {
	resourceList := urim.GetByAuthor(ID)

	deleteAll := len(routeID) == 0

	newRouteList := ResourceList[T]{}
	for _, resource := range resourceList {
		if deleteAll || routeID == resource.ID {
			uriSegmentList, _ := parseURI(resource.URI)
			segment := urim.root.GetSert(uriSegmentList)
			delete(segment.Data, ID)
		} else {
			newRouteList = append(newRouteList, resource)
		}
	}

	e := urim.storage.Set("resources", ID, newRouteList)
	return e
}

func (urim *URIM[T]) ClearByAuthor(ID string) error {
	e := urim.DeleteByAuthor(ID, "")
	return e
}
