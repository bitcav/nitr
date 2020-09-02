package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

const database string = "nitr.db"
const fileMode os.FileMode = 0600

//SetupDB creates nitr database with default values
func SetupDB() error {
	db, err := bolt.Open(database, fileMode, nil)

	if err != nil {
		return fmt.Errorf("could not open db, %v", err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return fmt.Errorf("could not create root bucket: %v", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not set up buckets, %v", err)
	}
	return nil
}

//SetUserData adds User data to nitr database with default values
func SetUserData(id string, user models.User) error {
	db, err := bolt.Open(database, fileMode, nil)

	if err != nil {
		return fmt.Errorf("could not open db, %v", err)
	}
	defer db.Close()

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

//GetUserByID returns User by ID
func GetUserByID(id string) models.User {
	db, err := bolt.Open(database, fileMode, nil)

	if err != nil {
		fmt.Println("could not open db")
	}

	defer db.Close()

	var userData models.User
	err = db.View(func(tx *bolt.Tx) error {
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

//GetApiKey returns current User Api Key
func GetApiKey() string {
	nitrUser := GetUserByID("1")
	return nitrUser.Apikey
}

func SetAPIData() {
	//DB Setup
	if _, err := os.Stat("nitr.db"); err != nil {
		log.Println("Database created")
		err := SetupDB()
		utils.LogError(err)

		log.Println("Adding default user")

		APIKey := utils.RandString(10)

		port := viper.GetString("port")
		if port == "" {
			port = "3000"
		}

		user := models.User{Username: "admin", Password: "admin", Apikey: APIKey}
		err = SetUserData("1", user)
		utils.LogError(err)
	}
}
