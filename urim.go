package wamp3router

import (
	"errors"
	"log"
	"regexp"
	"strings"

	wamp "github.com/wamp3hub/wamp3go"
)

var URI_RE, _ = regexp.Compile(`^(\*{1,2}|[_0-9a-z]+)(\.(\*{1,2}|[_0-9a-z]+))*$`)

func parseURI(v string) ([]string, error) {
	if !URI_RE.MatchString(v) {
		return nil, errors.New("InvalidURI")
	}

	result := strings.Split(v, ".")
	return result, nil
}

type ResourceList[T any] []*wamp.Resource[T]

type URIM[T any] struct {
	root    *URISegment[*wamp.Resource[T]]
	storage Storage
}

func NewURIM[T any](storage Storage) *URIM[T] {
	return &URIM[T]{newURISegment[*wamp.Resource[T]](nil), storage}
}

func (urim *URIM[T]) Add(resource *wamp.Resource[T]) error {
	path, e := parseURI(resource.URI)
	if e == nil {
		resourceList := urim.GetByAuthor(resource.AuthorID)

		newResourceList := append(resourceList, resource)

		e = urim.SetByAuthor(resource.AuthorID, newResourceList)
		if e == nil {
			segment := urim.root.GetSert(path)
			segment.Data[resource.ID] = resource
		}
	}
	return e
}

// returns empty slice if something went wrong
func (urim *URIM[T]) Match(uri string) ResourceList[T] {
	resourceList := ResourceList[T]{}

	path, e := parseURI(uri)
	if e == nil {
		for _, segment := range urim.root.Get(path) {
			for _, resource := range segment.Data {
				resourceList = append(resourceList, resource)
			}
		}
	}

	return resourceList
}

// returns empty list if something went wrong
func (urim *URIM[T]) GetByAuthor(ID string) ResourceList[T] {
	resourceList := ResourceList[T]{}

	e := urim.storage.Get("resources", ID, &resourceList)
	if e != nil {
		log.Printf("[urim] GetByAuthor %s", e)
	}

	return resourceList
}

func (urim *URIM[T]) SetByAuthor(ID string, newResourceList ResourceList[T]) error {
	e := urim.storage.Set("resources", ID, newResourceList)
	return e
}

func (urim *URIM[T]) DeleteByAuthor(ID string, routeID string) error {
	resourceList := urim.GetByAuthor(ID)

	deleteAll := len(routeID) == 0

	newResourceList := ResourceList[T]{}
	for _, resource := range resourceList {
		if deleteAll || routeID == resource.ID {
			path, _ := parseURI(resource.URI)
			segment := urim.root.GetSert(path)
			delete(segment.Data, resource.ID)
		} else {
			newResourceList = append(newResourceList, resource)
		}
	}

	e := urim.SetByAuthor(ID, newResourceList)
	return e
}

func (urim *URIM[T]) ClearByAuthor(ID string) error {
	e := urim.DeleteByAuthor(ID, "")
	return e
}

func (urim *URIM[T]) DumpURIList() []string {
	result := []string{}
	pathList := urim.root.PathDump()
	for _, path := range pathList {
		uri := strings.Join(path, ".")
		result = append(result, uri)
	}
	return result
}
