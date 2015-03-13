package main

import (
	"flag"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"log"
	"log/syslog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var name string = ""
var date BackupID = -1
var schedule string = ""
var full bool = false

type boolFlag interface {
	flag.Value
	IsBoolFlag() bool
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s %s [options]\n\nOptions:\n", programName, os.Args[0])
	flag.VisitAll(func(f *flag.Flag) {
		if f.Usage[0] == '-' {
			fmt.Fprintf(os.Stderr, "  -%s\n   alias for -%s\n\n", f.Name, f.Usage)
		} else {
			if fv, ok := f.Value.(boolFlag); ok && fv.IsBoolFlag() {
				fmt.Fprintf(os.Stderr, "  --%s[=true] or --%s=false\n   %s\n", f.Name, f.Name, f.Usage)
			} else {
				fmt.Fprintf(os.Stderr, "  --%s=%s\n   %s\n", f.Name, strings.ToUpper(f.Name), f.Usage)
			}
			if f.DefValue != "" {
				fmt.Fprintf(os.Stderr, "   default: %s\n", f.DefValue)
			}
			fmt.Fprintln(os.Stderr)
		}
	})
	os.Exit(1)
}

func printlist(l []string) {
	fmt.Print("[ ")
	for i, item := range l {
		if i != 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%q", item)
	}
	fmt.Println(" ]")
}

func config() {
	Setup()

	fmt.Println("# global configuration")
	if cfg.Server != "" {
		fmt.Printf("server = %q\n", cfg.Server)
	}
	if cfg.Port != 0 {
		fmt.Printf("port = %d\n", cfg.Port)
	}
	if cfg.User != "" {
		fmt.Printf("user = %q\n", cfg.User)
	}
	if cfg.IsServer() {
		fmt.Println("# server-side configuration")
		if cfg.Catalog != "" {
			fmt.Printf("catalog = %q\n", cfg.Catalog)
		}
		if cfg.Vault != "" {
			fmt.Printf("vault = %q\n", cfg.Vault)
		}
		if cfg.Maxtries != 0 {
			fmt.Printf("maxtries = %d\n", cfg.Maxtries)
		}
	}

	if len(cfg.Include) > 0 {
		fmt.Print("include = ")
		printlist(cfg.Include)
	}
	if len(cfg.Exclude) > 0 {
		fmt.Print("exclude = ")
		printlist(cfg.Exclude)
	}
}

func main() {
	if logwriter, err := syslog.New(syslog.LOG_NOTICE, filepath.Base(os.Args[0])); err == nil {
		log.SetOutput(logwriter)
		log.SetFlags(0) // no need to add timestamp, syslog will do it for us
	}

	setDefaults()

	flag.StringVar(&configFile, "config", defaultConfig, "Configuration file")
	flag.StringVar(&configFile, "c", defaultConfig, "-config")
	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.IntVar(&protocol, "protocol", protocol, "Protocol version")
	flag.IntVar(&protocol, "p", protocol, "-protocol")
	flag.IntVar(&timeout, "timeout", timeout, "Backend timeout (in seconds)")
	flag.IntVar(&timeout, "t", timeout, "-timeout")
	flag.Usage = usage

	programFile = os.Args[0]

	if len(os.Args) <= 1 { // no command specified
		os.Args = append(os.Args, defaultCommand)
	}

	if os.Args[1][0] != '-' {
		os.Args = os.Args[1:]
	} else {
		os.Args[0] = defaultCommand
	}

	if os.Args[0] == "help" {
		os.Args = append(os.Args, "-help")
		os.Args = os.Args[1:]
	}

	switch os.Args[0] {
	// client commands
	case "archive", "tar":
		archive()
	case "backup", "save":
		backup()
	case "expire":
		expire()
	case "info", "list":
		list()
	case "config", "cfg":
		config()
	case "ping", "test":
		ping()
	case "register":
		register()
	case "purge", "delete":
		purge()
	case "restore":
		restore()
	case "resume", "continue":
		resume()
	case "verify", "check":
		verify()
	case "web", "ui":
		web()
	// server commands
	case "data":
		data()
	case "df":
		df()
	case "vacuum":
		dbmaintenance()
	case "expirebackup":
		expirebackup()
	case "metadata":
		metadata()
	case "newbackup":
		newbackup()
	case "purgebackup":
		purgebackup()
	case "submitfiles":
		submitfiles()
	// shared commands
	case "help":
		fmt.Fprintf(os.Stderr, "Usage: %s help [command]", programName)
	case "-help", "--help", "-h":
		fmt.Printf("%s is a lightweight network backup system.\n\n", programName)
		fmt.Printf("Usage:\n\n\t%s COMMAND [options]\n\nCommands:\n", programName)
		fmt.Printf(`
    archive     retrieve files from backup
    backup      perform a new backup
    config      display configuration details
    expire      flush old backups
    info        display existing backups
    ping        check server connectivity
    purge       delete a backup
    register    send identity to the server
    restore     restore files from backup
    resume      continue a partial backup
    verify      verify a backup
    version     display version information
    web         start the built-in web server
`)
		fmt.Printf("\nUse \"%s help [command]\" for more information about a command.\n\n", programName)
	case "version":
		Setup()
		fmt.Printf("%s version %d.%d %s/%s\n", programName, versionMajor, versionMinor, runtime.GOOS, runtime.GOARCH)
		if verbose {
			fmt.Println()
			fmt.Println("Go version", runtime.Version())
			sqliteversion, _, _ := sqlite3.Version()
			fmt.Println("SQLite version", sqliteversion)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\nTry '--help' for more information.\n", os.Args[0])
		os.Exit(1)
	}
}
