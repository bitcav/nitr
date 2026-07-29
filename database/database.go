package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

const database string = "nitr.db"
const fileMode os.FileMode = 0600

// openTimeout bounds how long opening nitr.db waits to acquire its exclusive
// flock. bolt.Open's zero-value Timeout (what every call here used before)
// waits forever, so a CLI command (`nitr key`, `nitr passwd`) racing the
// running server -- or any two nitr processes pointed at the same
// data_dir -- hung with no output and no deadline instead of reporting the
// conflict. A var, not a const, so tests can shrink it instead of waiting
// out the production timeout.
var openTimeout = 5 * time.Second

var (
	dbMu         sync.Mutex
	dbHandle     *bolt.DB
	dbHandlePath string
)

// open returns the package's single *bolt.DB handle, opening nitr.db on
// first use instead of on every call. Every exported function below goes
// through it: GetApiKey runs on every API request (AuthAPI), and reopening
// plus re-mmapping the file that often is wasteful now and untenable once a
// background sampler is writing to it every few seconds.
//
// The handle is keyed by DBPath's current value rather than opened exactly
// once per process: DBPath can change at runtime (data_dir is a viper key,
// not a constant), and the test suite exercises that by changing directory
// between cases. A path change transparently closes the stale handle and
// opens the new one, so callers never see a handle pointed at the wrong
// file.
func open() (*bolt.DB, error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	path := DBPath()
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	if dbHandle != nil {
		if dbHandlePath == path {
			return dbHandle, nil
		}
		_ = dbHandle.Close()
		dbHandle = nil
	}

	// A configured data dir may not exist yet (fresh Docker volume); the
	// cwd default always does. Errors surface at bolt.Open with context.
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	db, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			return nil, fmt.Errorf("database is locked by another nitr process (waited %s): %s", openTimeout, path)
		}
		return nil, fmt.Errorf("could not open db, %w", err)
	}

	dbHandle = db
	dbHandlePath = path
	return dbHandle, nil
}

// Close releases the package's bbolt handle, if one is open. Safe to call
// when nothing was ever opened (e.g. `nitr version`, which touches no
// database) and safe to call more than once.
func Close() error {
	dbMu.Lock()
	defer dbMu.Unlock()
	if dbHandle == nil {
		return nil
	}
	err := dbHandle.Close()
	dbHandle = nil
	dbHandlePath = ""
	return err
}

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
	db, err := open()
	if err != nil {
		return err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return fmt.Errorf("could not create root bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not set up buckets, %w", err)
	}
	return nil
}

// SetUserData adds User data to nitr database with default values
func SetUserData(id string, user models.User) error {
	db, err := open()
	if err != nil {
		return err
	}

	userBytes, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("could not marshal entry json: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return fmt.Errorf("users bucket missing in %s: database is not initialised", DBPath())
		}
		err := b.Put([]byte(id), []byte(userBytes))
		if err != nil {
			return fmt.Errorf("could not insert entry: %w", err)
		}

		return nil
	})
	return err
}

// GetUserByID returns User by ID
func GetUserByID(id string) (models.User, error) {
	db, err := open()
	if err != nil {
		return models.User{}, err
	}

	var userData models.User
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return fmt.Errorf("users bucket missing in %s: database is not initialised", DBPath())
		}
		user := b.Get([]byte(id))
		if err := json.Unmarshal(user, &userData); err != nil {
			return fmt.Errorf("could not unmarshal user %q: %w", id, err)
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

// SetAPIData ensures nitr.db exists and holds a default user, provisioning
// both on first run. It returns an error rather than logging and continuing:
// the caller must refuse to start the server against a store it knows is
// broken, rather than serving traffic that will fail on every request.
func SetAPIData() error {
	// DB Setup: CreateBucketIfNotExists is safe to re-run, so a bucket-less
	// nitr.db (touched/restored empty) self-heals instead of panicking.
	_, statErr := os.Stat(DBPath())
	if err := SetupDB(); err != nil {
		return fmt.Errorf("setting up database: %w", err)
	}

	if statErr != nil {
		log.Println("Database created")
		log.Println("Adding default user")

		APIKey := utils.RandString(10)

		user := models.User{Password: utils.PasswordHash("123456"), Apikey: APIKey}
		if err := SetUserData("1", user); err != nil {
			return fmt.Errorf("provisioning default user: %w", err)
		}
	}
	return nil
}
