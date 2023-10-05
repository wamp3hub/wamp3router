package wamp3router

import (
	wamp "github.com/wamp3hub/wamp3go"
	clientShared "github.com/wamp3hub/wamp3go/shared"
)

type Newcomers = clientShared.Consumer[*wamp.Peer]

type Storage interface {
	Get(bucketName string, key string, data any) error
	Set(bucketName string, key string, data any) error
	Delete(bucketName string, key string)
	Destroy() error
}
