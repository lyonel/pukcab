package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server  string
	User    string
	Port    int
	Include []string
	Exclude []string

	Vault   string
	Catalog string

	Maxtries int
}

var cfg Config
var configFile string = defaultConfig

func loadconfig() {
	if _, err := toml.DecodeFile(configFile, &cfg); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Failed to parse configuration: ", err)
		log.Fatal("Failed to parse configuration: ", err)
	}

	if _, err := toml.DecodeFile(filepath.Join(os.Getenv("HOME"), defaultUserConfig), &cfg); err != nil && !os.IsNotExist(err) {
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

	if IsServer() {
		if pw, err := Getpwnam(cfg.User); err == nil {
			cfg.Exclude = append(cfg.Exclude, filepath.Join(pw.Dir, cfg.Vault), filepath.Join(pw.Dir, cfg.Catalog))
		}
	}
}

func IsServer() bool {
	return len(cfg.Server) < 1
}

func ServerOnly() {
	if !IsServer() {
		fmt.Println("This command can only be used on a", programName, "server.")
		log.Fatal("Server-only command issued on a client.")
	}
}

func ClientOnly() {
	if IsServer() {
		fmt.Println("This command can only be used on a", programName, "client.")
		log.Fatal("Client-only command issued on a server.")
	}
}

func Setup() {
	flag.Parse()
	loadconfig()

	if protocol > protocolVersion {
		fmt.Fprintln(os.Stderr, "Unsupported protocol")
		log.Fatalf("Protocol error (supported=%d requested=%d)", protocolVersion, protocol)
	}
}
