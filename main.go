package main

import (
	"flag"
	"fmt"
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
var age uint = 14
var full bool = false

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s %s [options]\n\nOptions:\n", programName, os.Args[0])
	flag.VisitAll(func(f *flag.Flag) {
		if f.Usage[0] == '-' {
			fmt.Fprintf(os.Stderr, "  -%s\n   alias for -%s\n\n", f.Name, f.Usage)
		} else {
			fmt.Fprintf(os.Stderr, "  --%s=%s\n   %s\n", f.Name, strings.ToUpper(f.Name), f.Usage)
			if f.DefValue != "" {
				fmt.Fprintf(os.Stderr, "   default: %s\n", f.DefValue)
			}
			fmt.Fprintln(os.Stderr)
		}
	})
	os.Exit(1)
}

func main() {
	if logwriter, err := syslog.New(syslog.LOG_NOTICE, filepath.Base(os.Args[0])); err == nil {
		log.SetOutput(logwriter)
		log.SetFlags(0) // no need to add timestamp, syslog will do it for us
	}

	setDefaults()

	flag.StringVar(&configFile, "config", defaultConfig, "Configuration file")
	flag.StringVar(&configFile, "c", defaultConfig, "-config")
	flag.Usage = usage

	loadconfig()

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
	case "backup":
		backup()
	case "newbackup":
		newbackup()
	case "info":
		info()
	case "backupinfo":
		backupinfo()
	case "submitfiles":
		submitfiles()
	case "ping":
		ping()
	case "expire":
		expire()
	case "help":
		fmt.Fprintf(os.Stderr, "Usage: %s help [command]", programName)
	case "-help", "--help", "-h":
		fmt.Fprintln(os.Stderr, "Usage: %s COMMAND [options]\n\nCommands:", programName)
		fmt.Fprintln(os.Stderr, "  backup")
		fmt.Fprintln(os.Stderr, "  info")
		fmt.Fprintln(os.Stderr, "  ping")
		fmt.Fprintln(os.Stderr, "  expire")
		fmt.Fprintln(os.Stderr, "  help")
	case "version":
		fmt.Printf("%s version %d.%d %s/%s\n", programName, versionMajor, versionMinor, runtime.GOOS, runtime.GOARCH)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\nTry '--help' for more information.\n", os.Args[0])
		os.Exit(1)
	}
}
