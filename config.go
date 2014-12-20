package main

import (
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server  string
	User    string
	Include []string
	Exclude []string

	Vault   string
	Catalog string
}

var cfg Config

func loadconfig() {
	if _, err := toml.DecodeFile(defaultConfig, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse configuration: ", err)
		log.Fatal("Failed to parse configuration: ", err)
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
}
