package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type SessionCookieSettings struct {
	Secure   bool   `json:"secure"`
	SameSite string `json:"same_site"`
}

var defaultSessionCookieSettings = SessionCookieSettings{
	Secure:   false,
	SameSite: "strict",
}

func init() {
	config.GlobalConfig.Register("session_cookie", &defaultSessionCookieSettings)
}

func GetSessionCookieSettings() *SessionCookieSettings {
	return &defaultSessionCookieSettings
}
