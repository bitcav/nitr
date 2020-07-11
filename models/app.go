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

type QR struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Port        string `json:"port"`
	Key         string `json:"key"`
}
