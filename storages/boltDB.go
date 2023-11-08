package routerStorages

import (
	"encoding/json"
	"errors"

	"github.com/boltdb/bolt"
)

type BoltDBStorage struct {
	super *bolt.DB
}

func NewBoltDBStorage(path string) (*BoltDBStorage, error) {
	db, e := bolt.Open(path, 0600, nil)
	if e != nil {
		// TODO log error
		return nil, e
	}

	storage := &BoltDBStorage{super: db}
	return storage, nil
}

func (storage *BoltDBStorage) Destroy() error {
	e := storage.super.Close()
	// TODO log error
	return e
}

func (storage *BoltDBStorage) Get(bucketName string, key string, data any) error {
	getRecord := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return errors.New("BucketNotFound")
		}
		raw := bucket.Get([]byte(key))
		if len(raw) == 0 {
			return errors.New("RecordNotFound")
		}
		e := json.Unmarshal(raw, data)
		return e
	}

	e := storage.super.View(getRecord)
	// TODO log error
	return e
}

func (storage *BoltDBStorage) Set(bucketName string, key string, data any) error {
	raw, e := json.Marshal(data)
	if e != nil {
		return e
	}

	putRecord := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			bucket, e = tx.CreateBucket([]byte(bucketName))
			if e != nil {
				return e
			}
		}
		e = bucket.Put([]byte(key), raw)
		return e
	}

	e = storage.super.Update(putRecord)
	// TODO log error
	return e
}

func (storage *BoltDBStorage) Delete(bucketName string, key string) {
	deleteRecord := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket != nil {
			bucket.Delete([]byte(key))
		}
		return nil
	}

	storage.super.Update(deleteRecord)
	// TODO log error
}
