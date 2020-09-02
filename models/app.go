package models

type Login struct {
	Password string `form:"password"`
}

type Password struct {
	CurrentPassword    string `form:"currentPassword"`
	NewPassword        string `form:"newPassword"`
	RepeateNewPassword string `form:"repeatNewPassword"`
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

type User struct {
	Password string
	Apikey   string
}
