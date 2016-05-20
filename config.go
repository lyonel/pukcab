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
	Command string
	Tar     string
	Port    int
	Include []string
	Exclude []string

	Vault   string
	Catalog string
	Web     string
	WebRoot string

	Maxtries int
	Debug    bool

	Expiration struct{ Daily, Weekly, Monthly, Yearly int64 }
}

var cfg Config
var configFile string = defaultConfig

func (cfg *Config) Load(filename string) {
	if _, err := toml.DecodeFile(filename, &cfg); err != nil && !os.IsNotExist(err) {
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
	if len(cfg.Command) < 1 {
		cfg.Command = programName
	}
	if len(cfg.Tar) < 1 {
		cfg.Tar = "tar"
	}

	if cfg.Maxtries < 1 {
		cfg.Maxtries = defaultMaxtries
	}

	if cfg.IsServer() {
		if pw, err := Getpwnam(cfg.User); err == nil {
			if filepath.IsAbs(cfg.Vault) {
				cfg.Exclude = append(cfg.Exclude, cfg.Vault)
			} else {
				cfg.Exclude = append(cfg.Exclude, filepath.Join(pw.Dir, cfg.Vault))
			}
		}
	}

	return
}

func (cfg *Config) IsServer() bool {
	return len(cfg.Server) < 1
}

func (cfg *Config) ServerOnly() {
	if !cfg.IsServer() {
		fmt.Println("This command can only be used on a", programName, "server.")
		log.Fatal("Server-only command issued on a client.")
	}
}

func (cfg *Config) ClientOnly() {
	if cfg.IsServer() {
		fmt.Println("This command can only be used on a", programName, "client.")
		log.Fatal("Client-only command issued on a server.")
	}
}

func Setup() {
	flag.Parse()
	Info(verbose)
	cfg.Load(configFile)
	Debug(cfg.Debug)

	if protocol > protocolVersion {
		fmt.Fprintln(os.Stderr, "Unsupported protocol")
		log.Fatalf("Protocol error (supported=%d requested=%d)", protocolVersion, protocol)
	}
}
