package nitrdb

import (
	"encoding/json"
	"fmt"
	"log"

	bolt "go.etcd.io/bbolt"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Apikey   string `json:"apikey"`
	QrCode   string `json:"qrCode"`
}

func SetupDB() (*bolt.DB, error) {
	db, err := bolt.Open("nitr.db", 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db, %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return fmt.Errorf("could not create root bucket: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not set up buckets, %v", err)
	}
	return db, nil
}

func SetUserData(db *bolt.DB, id string, user User) error {
	userBytes, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("could not marshal entry json: %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		err := tx.Bucket([]byte("users")).Put([]byte(id), []byte(userBytes))
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

func GetUserByID(db *bolt.DB, id string) User {
	var userData User
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		user := b.Get([]byte(id))
		if err := json.Unmarshal(user, &userData); err != nil {
			panic(err)
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return userData
}

func GetApiKey() string {
	db, err := bolt.Open("nitr.db", 0600, nil)

	if err != nil {
		fmt.Errorf("could not open db, %v", err)
	}
	nitrUser := GetUserByID(db, "1")
	db.Close()
	return nitrUser.Apikey

}
