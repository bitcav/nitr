package models

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bitcav/nitr-core/host"
	"github.com/stretchr/testify/assert"
)

func TestLoginJSON(t *testing.T) {
	l := Login{Password: "secret"}
	assert.Equal(t, "secret", l.Password)
}

func TestPasswordModelJSON(t *testing.T) {
	p := Password{
		CurrentPassword:   "a",
		NewPassword:       "b",
		RepeatNewPassword: "b",
	}
	assert.Equal(t, "a", p.CurrentPassword)
	assert.Equal(t, "b", p.NewPassword)
	assert.Equal(t, "b", p.RepeatNewPassword)
}

func TestApiKeyJSON(t *testing.T) {
	k := ApiKey{Key: "abc", QrCode: "{}"}
	data, err := json.Marshal(k)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"key":"abc","qrCode":"{}"}`, string(data))

	var out ApiKey
	assert.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, k, out)
}

func TestHostInfoQrCodeOmitempty(t *testing.T) {
	// QrCode is omitempty: absent when empty
	h := HostInfo{Name: "n", Description: "d", IP: "1.1.1.1", Port: "8000", Key: "k"}
	data, err := json.Marshal(h)
	assert.NoError(t, err)
	assert.NotContains(t, string(data), "qrCode")

	// present when set
	h.QrCode = "{}"
	data, err = json.Marshal(h)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "qrCode")
}

func TestNewHostInfoCallsHostInfoOnce(t *testing.T) {
	origHost, origIP, origPort := hostInfoFunc, getLocalIPFunc, getLocalPortFunc
	calls := 0
	hostInfoFunc = func() host.HostInfo {
		calls++
		return host.HostInfo{Name: "n", Platform: "p", Arch: "a"}
	}
	getLocalIPFunc = func() (string, error) { return "1.2.3.4", nil }
	// Distinctive test-owned value: proves the stub is in effect (a stubbed
	// "8000" could silently pass on the real fallback).
	getLocalPortFunc = func() string { return "9999" }
	t.Cleanup(func() { hostInfoFunc, getLocalIPFunc, getLocalPortFunc = origHost, origIP, origPort })

	info, err := NewHostInfo("k")
	assert.NoError(t, err)
	assert.Equal(t, 1, calls, "host.Info() must be called exactly once per NewHostInfo")
	assert.Equal(t, HostInfo{Name: "n", Description: "p/a", IP: "1.2.3.4", Port: "9999", Key: "k"}, info)
}

func TestNewHostInfoIPError(t *testing.T) {
	origHost, origIP, origPort := hostInfoFunc, getLocalIPFunc, getLocalPortFunc
	hostInfoFunc = func() host.HostInfo { return host.HostInfo{} }
	getLocalIPFunc = func() (string, error) { return "", errors.New("no route") }
	getLocalPortFunc = func() string { return "9999" }
	t.Cleanup(func() { hostInfoFunc, getLocalIPFunc, getLocalPortFunc = origHost, origIP, origPort })

	_, err := NewHostInfo("k")
	assert.Error(t, err)
}

func TestUserModel(t *testing.T) {
	u := User{Password: "p", Apikey: "a"}
	assert.Equal(t, "p", u.Password)
	assert.Equal(t, "a", u.Apikey)
}
