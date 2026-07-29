package database

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func cdTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	dir, err := ioutil.TempDir("", "nitrdbtest")
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		_ = os.RemoveAll(dir)
	})
	return dir
}

func TestSetupDB(t *testing.T) {
	cdTemp(t)

	err := SetupDB()
	require.NoError(t, err)
	assert.FileExists(t, "nitr.db")

	// idempotent: bucket already exists
	err = SetupDB()
	assert.NoError(t, err)
}

func TestSetupDBOpenError(t *testing.T) {
	cdTemp(t)

	// make nitr.db a directory so bolt.Open fails
	require.NoError(t, os.Mkdir("nitr.db", 0755))
	err := SetupDB()
	assert.Error(t, err)
}

func TestSetAndGetUserData(t *testing.T) {
	cdTemp(t)
	require.NoError(t, SetupDB())

	user := models.User{Password: "hashed", Apikey: "key-abc"}
	require.NoError(t, SetUserData("1", user))

	got, err := GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, "hashed", got.Password)
	assert.Equal(t, "key-abc", got.Apikey)
}

func TestSetUserDataOpenError(t *testing.T) {
	cdTemp(t)
	// nitr.db is a directory -> open fails
	require.NoError(t, os.Mkdir("nitr.db", 0755))
	err := SetUserData("1", models.User{})
	assert.Error(t, err)
}

func TestGetApiKey(t *testing.T) {
	cdTemp(t)
	require.NoError(t, SetupDB())
	require.NoError(t, SetUserData("1", models.User{Password: "p", Apikey: "superkey"}))
	key, err := GetApiKey()
	require.NoError(t, err)
	assert.Equal(t, "superkey", key)
}

func TestGetUserByIDMissingReturnsError(t *testing.T) {
	cdTemp(t)
	require.NoError(t, SetupDB())
	// bucket exists but key "missing" is absent -> json.Unmarshal(nil) errors
	_, err := GetUserByID("missing")
	assert.Error(t, err)
}

func TestGetUserByIDNoBucketReturnsError(t *testing.T) {
	cdTemp(t)
	// valid bbolt file with zero buckets (e.g. touched/restored empty nitr.db)
	db, err := bolt.Open("nitr.db", 0600, nil)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = GetUserByID("1")
	assert.Error(t, err)
}

func TestSetUserDataNoBucketReturnsError(t *testing.T) {
	cdTemp(t)
	db, err := bolt.Open("nitr.db", 0600, nil)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	err = SetUserData("1", models.User{})
	assert.Error(t, err)
}

func TestSetAPIDataHealsBucketlessDB(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	db, err := bolt.Open("nitr.db", 0600, nil)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	require.NoError(t, SetAPIData())
	require.NoError(t, SetUserData("1", models.User{Password: "p", Apikey: "k"}))
	_, err = GetUserByID("1")
	assert.NoError(t, err)
}

func TestSetAPIDataReturnsErrorWhenSetupFails(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	// nitr.db is a directory -> bolt.Open fails -> SetupDB errors -> SetAPIData
	// must propagate rather than swallow it.
	require.NoError(t, os.Mkdir("nitr.db", 0755))

	err := SetAPIData()
	assert.Error(t, err)
}

func TestGetUserByIDOpenError(t *testing.T) {
	cdTemp(t)
	// nitr.db is a directory -> bolt.Open fails -> GetUserByID returns an error
	require.NoError(t, os.Mkdir("nitr.db", 0755))
	_, err := GetUserByID("1")
	assert.Error(t, err)
}

func TestSetAPIDataFirstRun(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	// no nitr.db -> provisions default user (password 123456, random 10-char key)
	require.NoError(t, SetAPIData())
	assert.FileExists(t, "nitr.db")

	user, err := GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("123456"), user.Password)
	assert.Len(t, user.Apikey, 10)
}

func TestSetAPIDataSubsequentRunNoop(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	require.NoError(t, SetAPIData())
	first, err := GetApiKey()
	require.NoError(t, err)

	// second call must NOT re-provision (db already present)
	require.NoError(t, SetAPIData())
	key, err := GetApiKey()
	require.NoError(t, err)
	assert.Equal(t, first, key)
}

// TestDataDirPutsDBThere is the guard for --data-dir / NITR_DATA_DIR /
// data_dir: with data_dir set, the database file is created (including the
// missing directory) and read back from THERE — not the cwd. The default
// path is covered by every other test here, which all resolve to
// ./nitr.db.
func TestDataDirPutsDBThere(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	t.Cleanup(viper.Reset) // data_dir must not leak into the cwd-default tests

	dir := filepath.Join("nested", "data")
	viper.Set("data_dir", dir)

	require.NoError(t, SetAPIData())
	dbFile := filepath.Join(dir, "nitr.db")
	assert.FileExists(t, dbFile)
	assert.NoFileExists(t, "nitr.db", "with data_dir set, nothing may be written to the cwd")

	user, err := GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("123456"), user.Password)

	// DBPath is the single resolution point and must reflect the key.
	assert.Equal(t, dbFile, DBPath())
	viper.Set("data_dir", "")
	assert.Equal(t, "nitr.db", DBPath())
}
