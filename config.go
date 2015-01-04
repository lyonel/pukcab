package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server  string
	User    string
	Include []string
	Exclude []string

	Vault   string
	Catalog string

	Maxtries int
}

var cfg Config
var configFile string = defaultConfig

func loadconfig() {
	if _, err := toml.DecodeFile(configFile, &cfg); err != nil && err != os.ErrNotExist {
		fmt.Fprintln(os.Stderr, "Failed to parse configuration: ", err)
		log.Fatal("Failed to parse configuration: ", err)
	}

	if _, err := toml.DecodeFile(filepath.Join(os.Getenv("HOME"), defaultUserConfig), &cfg); err != nil && err != os.ErrNotExist {
		fmt.Fprintln(os.Stderr, "Failed to parse configuration: ", err)
		log.Fatal("Failed to parse configuration:", err)
	}

	if len(cfg.Include) < 1 {
		cfg.Include = defaultInclude
	}
	if len(cfg.Exclude) < 1 {
		cfg.Exclude = defaultExclude
	}
	if len(cfg.Vault) < 1 {
		cfg.Vault = defaultVault
	}
	if len(cfg.Catalog) < 1 {
		cfg.Catalog = defaultCatalog
	}

	if cfg.Maxtries < 1 {
		cfg.Maxtries = defaultMaxtries
	}
}
