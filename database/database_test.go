package database

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	SetAPIData()
	assert.FileExists(t, "nitr.db")

	user, err := GetUserByID("1")
	require.NoError(t, err)
	assert.Equal(t, utils.PasswordHash("123456"), user.Password)
	assert.Len(t, user.Apikey, 10)
}

func TestSetAPIDataSubsequentRunNoop(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	SetAPIData()
	first, err := GetApiKey()
	require.NoError(t, err)

	// second call must NOT re-provision (db already present)
	SetAPIData()
	key, err := GetApiKey()
	require.NoError(t, err)
	assert.Equal(t, first, key)
}
