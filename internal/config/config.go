package config

import (
	"log"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const (
	// Prefix indicates environment variables prefix.
	Prefix = "myapp_"
)

type (
	Config struct {
		API        API              `koanf:"api"`
		MSTeams    MSTeamsConfig    `koanf:"msteams"`
		Mattermost MattermostConfig `koanf:"mattermost"`
	}

	API struct {
		Port int `koanf:"port"`
	}

	MSTeamsConfig struct {
		URL         string `koanf:"url"`
		RuntimeURL  string `koanf:"runtime_url"`
		PlatformURL string `koanf:"platform_url"`
		NetworkURL  string `koanf:"network_url"`
	}

	MattermostConfig struct {
		Webhook string `koanf:"webhook"`
	}
)

func New() Config {
	var instance Config

	k := koanf.New(".")

	// load default configuration from file
	if err := k.Load(structs.Provider(Default(), "koanf"), nil); err != nil {
		log.Fatalf("error loading default: %s", err)
	}

	// load configuration from file
	if err := k.Load(file.Provider("config.yml"), yaml.Parser()); err != nil {
		log.Printf("error loading config.yml: %s", err)
	}

	// load environment variables
	if err := k.Load(env.Provider(Prefix, ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, Prefix)), "_", ".")
	}), nil); err != nil {
		log.Printf("error loading environment variables: %s", err)
	}

	if err := k.Unmarshal("", &instance); err != nil {
		log.Fatalf("error unmarshalling config: %s", err)
	}

	log.Printf("following configuration is loaded:\n%+v", instance)

	return instance
}
