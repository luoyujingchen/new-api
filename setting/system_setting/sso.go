package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type SSOSettings struct {
	DefaultProvider string `json:"default_provider"`
}

var defaultSSOSettings = SSOSettings{}

func init() {
	config.GlobalConfig.Register("sso", &defaultSSOSettings)
}

func GetSSOSettings() *SSOSettings {
	return &defaultSSOSettings
}
