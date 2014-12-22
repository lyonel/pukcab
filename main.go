package main

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"path/filepath"
	"strings"
)

var name string = "hostname"
var date int64 = -1
var schedule string = defaultSchedule
var age uint = 14
var full bool = false

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s %s [options]\n\nOptions:\n", programName, os.Args[0])
	flag.VisitAll(func(f *flag.Flag) {
		if f.Usage[0] == '-' {
			fmt.Fprintf(os.Stderr, "  -%s %s\n   alias for %s\n\n", f.Name, strings.ToUpper(f.Usage[1:]), f.Usage)
		} else {
			fmt.Fprintf(os.Stderr, "  -%s %s\n   %s\n   default: %q\n\n", f.Name, strings.ToUpper(f.Name), f.Usage, f.DefValue)
		}
	})
	os.Exit(1)
}

func main() {
	if logwriter, err := syslog.New(syslog.LOG_NOTICE, filepath.Base(os.Args[0])); err == nil {
		log.SetOutput(logwriter)
		log.SetFlags(0) // no need to add timestamp, syslog will do it for us
	}

	name, _ = os.Hostname()
	flag.StringVar(&name, "name", name, "Backup name")
	flag.StringVar(&name, "n", name, "-name")
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
	case "expire":
		expire()
	case "help":
		fmt.Fprintf(os.Stderr, "Usage: %s help [command]", programName)
	case "-help", "--help", "-h":
		fmt.Fprintln(os.Stderr, "Usage: %s COMMAND [options]\n\nCommands:", programName)
		fmt.Fprintln(os.Stderr, "  backup")
		fmt.Fprintln(os.Stderr, "  info")
		fmt.Fprintln(os.Stderr, "  expire")
		fmt.Fprintln(os.Stderr, "  help")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\nTry '--help' for more information.\n", os.Args[0])
		os.Exit(1)
	}
}
