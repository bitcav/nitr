package database

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestOpenReusesHandleAcrossCalls is the core of this ticket: two calls that
// resolve to the same nitr.db must share one *bolt.DB instead of each
// opening (and closing) their own. Against the old per-call
// bolt.Open/defer db.Close() code, SetupDB and GetUserByID would return
// different *bolt.DB pointers here.
func TestOpenReusesHandleAcrossCalls(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })

	require.NoError(t, SetupDB())
	first := dbHandle
	require.NotNil(t, first)

	require.NoError(t, SetUserData("1", models.User{Password: "p", Apikey: "k"}))
	_, err := GetUserByID("1")
	require.NoError(t, err)

	assert.Same(t, first, dbHandle, "GetUserByID/SetUserData must reuse SetupDB's handle, not open their own")
}

// TestOpenSwitchesHandleOnPathChange proves a data_dir change mid-process
// (the CLI and server both resolve DBPath from viper, which can change at
// runtime) closes the stale handle and opens the new path, rather than
// silently continuing to serve the old file.
func TestOpenSwitchesHandleOnPathChange(t *testing.T) {
	cdTemp(t)
	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
		_ = Close()
	})

	require.NoError(t, SetupDB())
	firstPath := dbHandlePath

	viper.Set("data_dir", "elsewhere")
	require.NoError(t, SetupDB())

	assert.NotEqual(t, firstPath, dbHandlePath)
	assert.Contains(t, dbHandlePath, "elsewhere")
	assert.FileExists(t, filepath.Join("elsewhere", "nitr.db"))
}

// TestOpenTimesOutWhenLocked proves the fix for bolt.Open's old nil-options
// call (Timeout 0 => wait forever): with another handle already holding
// nitr.db's exclusive flock, open() must fail within openTimeout with a
// message identifying the conflict, not hang indefinitely. Against the old
// code (bolt.Open(DBPath(), fileMode, nil) with no Options), this test would
// never return.
func TestOpenTimesOutWhenLocked(t *testing.T) {
	cdTemp(t)
	origTimeout := openTimeout
	openTimeout = 200 * time.Millisecond
	t.Cleanup(func() { openTimeout = origTimeout })

	holder, err := bolt.Open("nitr.db", 0600, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Close() })

	start := time.Now()
	err = SetupDB()
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked by another nitr process")
	assert.Less(t, elapsed, 2*time.Second, "open() must not block past openTimeout")
}

// TestCloseReleasesLockForAnotherOpener proves Close() actually releases
// nitr.db's flock rather than just forgetting the in-memory pointer: a
// bolt.Open that would otherwise contend with our handle must succeed
// immediately once Close() has run.
func TestCloseReleasesLockForAnotherOpener(t *testing.T) {
	cdTemp(t)

	require.NoError(t, SetupDB())
	require.NoError(t, Close())

	other, err := bolt.Open("nitr.db", 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err, "Close() must release the flock so another opener does not contend")
	require.NoError(t, other.Close())
}

// TestCloseThenReopenSucceeds proves the package can reopen after Close():
// the shutdown path (main.program.Stop) closes the handle, but the server
// process itself keeps running its own tests/CLI commands afterward and
// must not be left permanently unable to reach the database.
func TestCloseThenReopenSucceeds(t *testing.T) {
	cdTemp(t)
	t.Cleanup(func() { _ = Close() })

	require.NoError(t, SetupDB())
	require.NoError(t, Close())
	assert.Nil(t, dbHandle)

	require.NoError(t, SetupDB())
	assert.NotNil(t, dbHandle)
	_, err := GetUserByID("1")
	assert.Error(t, err) // no user provisioned, but proves the reopened handle works
}
