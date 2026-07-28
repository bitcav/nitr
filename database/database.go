package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

const database string = "nitr.db"
const fileMode os.FileMode = 0600

// DBPath is the single resolution point for nitr.db's location: the
// data_dir key (--data-dir flag / NITR_DATA_DIR env / data_dir in the
// config file) joined with the file name, or the bare file name — i.e. the
// working directory, exactly as before — when data_dir is unset. Every open
// and stat of the database goes through here so the location can never
// drift between call sites.
func DBPath() string {
	if dir := viper.GetString("data_dir"); dir != "" {
		return filepath.Join(dir, database)
	}
	return database
}

// SetupDB creates nitr database with default values
func SetupDB() error {
	// A configured data dir may not exist yet (fresh Docker volume); the
	// cwd default always does. Errors surface at bolt.Open with context.
	if dir := filepath.Dir(DBPath()); dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	db, err := bolt.Open(DBPath(), fileMode, nil)

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

// SetUserData adds User data to nitr database with default values
func SetUserData(id string, user models.User) error {
	db, err := bolt.Open(DBPath(), fileMode, nil)

	if err != nil {
		return fmt.Errorf("could not open db, %v", err)
	}
	defer db.Close()

	userBytes, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("could not marshal entry json: %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return fmt.Errorf("users bucket missing in %s: database is not initialised", DBPath())
		}
		err := b.Put([]byte(id), []byte(userBytes))
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})
	return err
}

// GetUserByID returns User by ID
func GetUserByID(id string) (models.User, error) {
	db, err := bolt.Open(DBPath(), fileMode, nil)

	if err != nil {
		return models.User{}, fmt.Errorf("could not open db, %v", err)
	}

	defer db.Close()

	var userData models.User
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return fmt.Errorf("users bucket missing in %s: database is not initialised", DBPath())
		}
		user := b.Get([]byte(id))
		if err := json.Unmarshal(user, &userData); err != nil {
			return fmt.Errorf("could not unmarshal user %q: %v", id, err)
		}

		return nil
	})
	if err != nil {
		return models.User{}, err
	}
	return userData, nil
}

// GetApiKey returns current User Api Key
func GetApiKey() (string, error) {
	nitrUser, err := GetUserByID("1")
	if err != nil {
		return "", err
	}
	return nitrUser.Apikey, nil
}

func SetAPIData() {
	// DB Setup: CreateBucketIfNotExists is safe to re-run, so a bucket-less
	// nitr.db (touched/restored empty) self-heals instead of panicking.
	_, statErr := os.Stat(DBPath())
	err := SetupDB()
	utils.LogError(err)

	if statErr != nil {
		log.Println("Database created")
		log.Println("Adding default user")

		APIKey := utils.RandString(10)

		port := viper.GetString("port")
		if port == "" {
			port = "3000"
		}

		user := models.User{Password: utils.PasswordHash("123456"), Apikey: APIKey}
		err = SetUserData("1", user)
		utils.LogError(err)
	}
}
