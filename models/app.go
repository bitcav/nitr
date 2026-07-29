package models

import (
	"github.com/bitcav/nitr-core/host"
	"github.com/bitcav/nitr/utils"
)

type Login struct {
	Password string `form:"password"`
}

type Password struct {
	CurrentPassword   string `form:"currentPassword"`
	NewPassword       string `form:"newPassword"`
	RepeatNewPassword string `form:"repeatNewPassword"`
}

type ApiKey struct {
	Key    string `json:"key"`
	QrCode string `json:"qrCode"`
}

type HostInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IP          string `json:"ip"`
	Port        string `json:"port"`
	Key         string `json:"key"`
	QrCode      string `json:"qrCode,omitempty"`
}

// hostInfoFunc, getLocalIPFunc and getLocalPortFunc are seams so tests can
// stub host.Info(), utils.GetLocalIP() and utils.GetLocalPort() without
// reading real host facts, network topology or global viper state; mirrors
// the bandwidthInfoFunc/ispInfoFunc seams in handlers/api.go.
var (
	hostInfoFunc     = host.Info
	getLocalIPFunc   = utils.GetLocalIP
	getLocalPortFunc = utils.GetLocalPort
)

// NewHostInfo builds a HostInfo for the given API key. host.Info() re-reads
// host facts (~800µs) on every call, so it is called exactly once and the
// result shared across fields — the three call sites this replaces each
// called it three times.
func NewHostInfo(key string) (HostInfo, error) {
	h := hostInfoFunc()
	localIP, err := getLocalIPFunc()
	if err != nil {
		return HostInfo{}, err
	}
	return HostInfo{
		Name:        h.Name,
		Description: h.Platform + "/" + h.Arch,
		IP:          localIP,
		Port:        getLocalPortFunc(),
		Key:         key,
	}, nil
}

type User struct {
	Password string
	Apikey   string
}
