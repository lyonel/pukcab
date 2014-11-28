package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const defaultCommand = "help"
const defaultSchedule = "daily"
const defaultConfig = "/etc/pukcab.conf"

type Config struct {
	Server string
	Include []string
	Exclude []string

	Vault   string
	Catalog string
}

var cfg Config

var name string = "hostname"
var date int64 = -1
var schedule string = defaultSchedule
var age uint = 14

func expire() {
	flag.Int64Var(&date, "date", date, "Backup set")
	flag.Int64Var(&date, "d", date, "-date")
	flag.UintVar(&age, "age", age, "Age")
	flag.UintVar(&age, "a", age, "-age")
	flag.Parse()

	log.Println("name: ", name)
	log.Println("date: ", date)
	log.Println("schedule: ", schedule)
	log.Println("age: ", age)
}

func newbackup() {
	flag.Parse()

	date = time.Now().Unix()
	log.Println("name: ", name)
	log.Println("date: ", date)
	log.Println("schedule: ", schedule)
	log.Println("server: ", cfg.Server)
	log.Println("include: ", cfg.Include)
	log.Println("exclude: ", cfg.Exclude)
}

func submitfiles() {
	flag.Int64Var(&date, "date", date, "Backup set")
	flag.Int64Var(&date, "d", date, "-date")
	flag.Parse()

	log.Println("name: ", name)
	log.Println("date: ", date)
	log.Println("schedule: ", schedule)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: pukcab %s [options]\n\nOptions:\n", os.Args[0])
	flag.VisitAll(func(f *flag.Flag) {
		if f.Usage[0] == '-' {
			fmt.Fprintf(os.Stderr, "  -%s %s\n   alias for \"%s %s\"\n\n", f.Name, strings.ToUpper(f.Usage[1:]), f.Usage, strings.ToUpper(f.Usage[1:]))
		} else {
			fmt.Fprintf(os.Stderr, "  -%s %s\n   %s\n   default: %q\n\n", f.Name, strings.ToUpper(f.Name), f.Usage, f.DefValue)
		}
	})
	os.Exit(1)
}

func loadconfig() {
	if _, err := toml.DecodeFile(defaultConfig, &cfg); err != nil {
		log.Fatal("Failed to parse configuration: ", err)
	}
}

func main() {
	name, _ = os.Hostname()
	flag.StringVar(&name, "name", name, "Backup name")
	flag.StringVar(&name, "n", name, "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
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
	case "newbackup":
		newbackup()
	case "submitfiles":
		submitfiles()
	case "expire":
		expire()
	case "help":
		fmt.Fprintf(os.Stderr, "Usage: pukcab help [command]")
	case "-help", "--help", "-h":
		fmt.Fprintln(os.Stderr, "Usage: pukcab COMMAND [options]\n\nCommands:")
		fmt.Fprintln(os.Stderr, "  newbackup")
		fmt.Fprintln(os.Stderr, "  expire")
		fmt.Fprintln(os.Stderr, "  help")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\nTry '--help' for more information.\n", os.Args[0])
		os.Exit(1)
	}
}
