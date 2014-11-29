package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/antage/mntent"
)

const defaultCommand = "help"
const defaultSchedule = "daily"
const defaultConfig = "/etc/pukcab.conf"

type Config struct {
	Server  string
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

func contains(set []string, e string) bool {
	for _, a := range set {
		if a == e {
			return true
		}

		if filepath.IsAbs(a) {
			if strings.HasPrefix(e, a+string(filepath.Separator)) {
				return true
			}
		} else {
			if matched, _ := filepath.Match(a, filepath.Base(e)); matched {
				return true
			}

			if strings.HasPrefix(a, "."+string(filepath.Separator)) {
				if _, err := os.Lstat(filepath.Join(e, a)); !os.IsNotExist(err) {
					return true
				}
			}
		}
	}
	return false
}

func included(e *mntent.Entry) bool {
	return !(contains(cfg.Exclude, e.Types[0]) || contains(cfg.Exclude, e.Directory)) && (contains(cfg.Include, e.Types[0]) || contains(cfg.Include, e.Directory))
}

func backup() {
	flag.Parse()

	date = time.Now().Unix()
	log.Println("name: ", name)
	log.Println("date: ", date)
	log.Println("schedule: ", schedule)
	log.Println("server: ", cfg.Server)
	log.Println("include: ", cfg.Include)
	log.Println("exclude: ", cfg.Exclude)

	directories := make(map[string]bool)
	devices := make(map[string]bool)

	if mtab, err := mntent.Parse("/etc/mtab"); err != nil {
		log.Println("Failed to parse /etc/mtab: ", err)
	} else {
		for i := range mtab {
			if !devices[mtab[i].Name] && included(mtab[i]) {
				devices[mtab[i].Name] = true
				directories[mtab[i].Directory] = true
			}
		}
	}

	for d := range directories {
		fmt.Println(d)
	}
}

func newbackup() {
	flag.Parse()

	date = time.Now().Unix()
	log.Println("name: ", name)
	log.Println("date: ", date)
	log.Println("schedule: ", schedule)
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

	if len(cfg.Include) < 1 {
		cfg.Include = []string{"ext2", "ext3", "ext4", "btrfs", "xfs", "jfs", "vfat"}
	}
	if len(cfg.Exclude) < 1 {
		cfg.Exclude = []string{"/proc", "/sys", "/selinux", "tmpfs"}
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
	case "backup":
		backup()
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