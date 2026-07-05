package plugin

type TogglePluginReq struct {
	Enabled bool `json:"enabled"`
}

type SaveSettingsReq struct {
	Settings map[string]interface{} `json:"settings"`
}
