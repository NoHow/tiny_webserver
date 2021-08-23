package main

import (
	"fmt"
	"github.com/boltdb/bolt"
	"log"
)

func InitDB() {
	db, err := bolt.Open("data/tws.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("PagesData"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}

func GetPage(title string) ([]byte, error) {
	db, err := bolt.Open("data/tws.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var resultData []byte
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("PagesData"))
		getResult := b.Get([]byte(title))
		resultData = append(resultData, getResult...)
		return nil
	})

	return resultData, err
}

func SavePage(title string, data []byte) error {
	db, err := bolt.Open("data/tws.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("PagesData"))
		return bucket.Put([]byte(title), data)
	})
}
