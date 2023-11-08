package routerShared

import (
	"errors"
	"log"
	"regexp"
	"strings"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
)

type Storage interface {
	Get(bucketName string, key string, data any) error
	Set(bucketName string, key string, data any) error
	Delete(bucketName string, key string)
	Destroy() error
}

var URI_RE, _ = regexp.Compile(`^(\*{1,2}|[_0-9a-z]+)(\.(\*{1,2}|[_0-9a-z]+))*$`)

func ParseURI(v string) ([]string, error) {
	if !URI_RE.MatchString(v) {
		return nil, errors.New("InvalidURI")
	}

	result := strings.Split(v, ".")
	return result, nil
}

type URIM[T any] struct {
	bucket  string
	root    *URISegment[*wamp.Resource[T]]
	storage Storage
}

func NewURIM[T any](storage Storage) *URIM[T] {
	return &URIM[T]{
		wampShared.NewID(),
		NewURISegment[*wamp.Resource[T]](nil),
		storage,
	}
}

type ResourceList[T any] []*wamp.Resource[T]

// returns empty slice if something went wrong
func (urim *URIM[T]) Match(uri string) ResourceList[T] {
	resourceList := ResourceList[T]{}
	path, e := ParseURI(uri)
	if e == nil {
		for _, segment := range urim.root.Match(path) {
			for _, resource := range segment.Data {
				resourceList = append(resourceList, resource)
			}
		}
	}
	return resourceList
}

func (urim *URIM[T]) Count(uri string) int {
	resourceList := urim.Match(uri)
	return len(resourceList)
}

// returns empty slice if something went wrong
func (urim *URIM[T]) GetByAuthor(ID string) ResourceList[T] {
	resourceList := ResourceList[T]{}
	e := urim.storage.Get(urim.bucket, ID, &resourceList)
	if e != nil {
		log.Printf("[urim] GetByAuthor %s", e)
	}
	return resourceList
}

func (urim *URIM[T]) setByAuthor(ID string, newResourceList ResourceList[T]) error {
	if len(newResourceList) == 0 {
		urim.storage.Delete(urim.bucket, ID)
		return nil
	}

	e := urim.storage.Set(urim.bucket, ID, newResourceList)
	return e
}

func (urim *URIM[T]) DeleteByAuthor(ID string, resourceID string) ResourceList[T] {
	shouldRemove := func(resource *wamp.Resource[T]) bool {
		return len(resourceID) == 0 || resourceID == resource.ID
	}

	resourceList := urim.GetByAuthor(ID)
	removedResourceList := ResourceList[T]{}
	newResourceList := ResourceList[T]{}
	for _, resource := range resourceList {
		if shouldRemove(resource) {
			path, _ := ParseURI(resource.URI)
			segment := urim.root.Get(path)
			delete(segment.Data, resource.ID)
			removedResourceList = append(removedResourceList, resource)
		} else {
			newResourceList = append(newResourceList, resource)
		}
	}

	e := urim.setByAuthor(ID, newResourceList)
	if e != nil {
		log.Printf("[urim] setByAuthor %s", e)
	}

	return removedResourceList
}

func (urim *URIM[T]) Add(resource *wamp.Resource[T]) error {
	path, e := ParseURI(resource.URI)
	if e == nil {
		resourceList := urim.GetByAuthor(resource.AuthorID)
		newResourceList := append(resourceList, resource)
		e = urim.setByAuthor(resource.AuthorID, newResourceList)
		if e == nil {
			segment := urim.root.GetSert(path)
			segment.Data[resource.ID] = resource
		}
	}
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
