package main

import (
	//"fmt"
	"bytes"

	"github.com/boltdb/bolt"
)

//DataStore - Default datastore interface
type DataStore interface {
	Init()
	Close()
	Get(key string) (val []byte, err error)
	GetByPrefix(prefix string) (kv map[string][]byte, err error)
	Put(key string, val []byte) error
	Puts(kv map[string][]byte) error
	Del(key string) error
	Dels(keys []string) error
	ifExist(key string) (exist bool, err error)
}

//FileDataStore - File Datastore
type FileDataStore struct {
	Path          string
	DefaultBucket string
	db            *bolt.DB
}

//Init - Initialize file based datastore
func (f *FileDataStore) Init() {
	var err error
	f.db, err = bolt.Open(f.Path, 0600, nil)
	if err != nil {
		panic(err)
	}

	if f.DefaultBucket == "" {
		f.DefaultBucket = "data"
	}

	err = f.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(f.DefaultBucket))
		return err
	})
	if err != nil {
		panic(err)
	}
}

//Close - Close file based datastore
func (f *FileDataStore) Close() {
	f.db.Close()
}

//Get - Get file based datastore
func (f *FileDataStore) Get(key string) (val []byte, err error) {
	err = f.db.View(func(tx *bolt.Tx) error {
		val = tx.Bucket([]byte(f.DefaultBucket)).Get([]byte(key))
		return nil
	})
	return val, err
}

//GetByPrefix - Get keys by prefix for file based datastore
func (f *FileDataStore) GetByPrefix(prefix string) (kv map[string][]byte, err error) {
	kv = make(map[string][]byte)
	err = f.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(f.DefaultBucket)).Cursor()
		//prefixBytes := []byte(prefix)
		for k, v := c.Seek([]byte(prefix)); k != nil && bytes.HasPrefix(k, []byte(prefix)); k, v = c.Next() {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			//vals = append(vals, v)
			kv[string(k)] = v
		}
		return nil
	})

	return kv, err
}

//Put - Put key in datastore
func (f *FileDataStore) Put(key string, val []byte) error {
	err := f.db.Update(func(tx *bolt.Tx) error {
		//err := tx.Bucket([]byte("data")).Put([]byte(key), buf.Bytes())
		err := tx.Bucket([]byte(f.DefaultBucket)).Put([]byte(key), val)
		return err
	})
	if err != nil {
		panic(err)
	}

	return err
}

//Puts - Puts keys in datastore
func (f *FileDataStore) Puts(kv map[string][]byte) error {
	err := f.db.Update(func(tx *bolt.Tx) error {
		//err := tx.Bucket([]byte("data")).Put([]byte(key), buf.Bytes())
		for key, val := range kv {
			err := tx.Bucket([]byte(f.DefaultBucket)).Put([]byte(key), val)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return err
}

//Del - Delete key from datastore
func (f *FileDataStore) Del(key string) error {
	err := f.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(f.DefaultBucket)).Delete([]byte(key))
	})
	return err
}

//Dels - Delete keys from datastore
func (f *FileDataStore) Dels(keys []string) error {
	err := f.db.Update(func(tx *bolt.Tx) error {
		for _, key := range keys {
			err := tx.Bucket([]byte(f.DefaultBucket)).Delete([]byte(key))
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

//ifExist - Check if key exist in datastore
func (f *FileDataStore) ifExist(key string) (exist bool, err error) {
	var val []byte
	err = f.db.View(func(tx *bolt.Tx) error {
		val = tx.Bucket([]byte(f.DefaultBucket)).Get([]byte(key))
		return nil
	})
	if val == nil {
		return false, err
	}
	return true, err
}

//End File Datastore

/*
//Redis Datastore
type RedisDataStore struct {
  Host string
  mux sync.Mutex
}

func (r *RedisDataStore) Init() {
  fmt.Println(r.Host)
}

func (r *RedisDataStore) Close() {
  fmt.Println(r.Host)
}
//End Redis Datastore
*/
