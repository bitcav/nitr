package models

type Login struct {
	Username string `form:"username"`
	Password string `form:"password"`
	Remember string `form:"remember"`
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
	Username string `json:"username"`
	Password string `json:"password"`
	Apikey   string `json:"apikey"`
}
