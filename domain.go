package wamp3router

type Storage interface {
	Get(bucketName string, key string, data any) error
	Set(bucketName string, key string, data any) error
	Delete(bucketName string, key string)
	Destroy() error
}
