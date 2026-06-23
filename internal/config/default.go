package config

func Default() Config {
	return Config{
		API: API{Port: 8080},
		MSTeams: MSTeamsConfig{
			URL:         "",
			RuntimeURL:  "https://...",
			PlatformURL: "https://...",
			NetworkURL:  "https://...",
		},
		Mattermost: MattermostConfig{
			Webhook: "",
		},
	}
}
