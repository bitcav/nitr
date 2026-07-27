package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginJSON(t *testing.T) {
	l := Login{Password: "secret"}
	assert.Equal(t, "secret", l.Password)
}

func TestPasswordModelJSON(t *testing.T) {
	p := Password{
		CurrentPassword:    "a",
		NewPassword:        "b",
		RepeateNewPassword: "b",
	}
	assert.Equal(t, "a", p.CurrentPassword)
	assert.Equal(t, "b", p.NewPassword)
	assert.Equal(t, "b", p.RepeateNewPassword)
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

func TestUserModel(t *testing.T) {
	u := User{Password: "p", Apikey: "a"}
	assert.Equal(t, "p", u.Password)
	assert.Equal(t, "a", u.Apikey)
}
