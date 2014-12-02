package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"database/sql"

	"github.com/BurntSushi/toml"
	"github.com/antage/mntent"
	_ "github.com/mattn/go-sqlite3"
)

const programName = "pukcab"
const defaultCommand = "help"
const defaultSchedule = "daily"
const defaultConfig = "/etc/pukcab.conf"

type Config struct {
	Server  string
	User    string
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

var directories map[string]bool
var backupset map[string]struct{}

var catalog *sql.DB

func remotecommand(arg ...string) *exec.Cmd {
	os.Setenv("SSH_CLIENT", "")
	os.Setenv("SSH_CONNECTION", "")

	if cfg.Server != "" {
		cmd := []string{"-q", "-C", "-oBatchMode=yes", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null"}
		if cfg.User != "" {
			cmd = append(cmd, "-l", cfg.User)
		}
		cmd = append(cmd, cfg.Server)
		cmd = append(cmd, programName)
		cmd = append(cmd, arg...)
		return exec.Command("ssh", cmd...)
	} else {

		if exe, err := os.Readlink("/proc/self/exe"); err == nil {
			return exec.Command(exe, arg...)
		} else {
			return exec.Command(programName, arg...)
		}
	}
}

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

func includeorexclude(e *mntent.Entry) bool {
	result := !(contains(cfg.Exclude, e.Types[0]) || contains(cfg.Exclude, e.Directory)) && (contains(cfg.Include, e.Types[0]) || contains(cfg.Include, e.Directory))

	directories[e.Directory] = result
	return result
}

func excluded(f string) bool {
	if _, known := directories[f]; known {
		return !directories[f]
	}
	return contains(cfg.Exclude, f) && !contains(cfg.Include, f)
}

func addfiles(d string) {
	backupset[d] = struct{}{}
	files, _ := ioutil.ReadDir(d)
	for _, f := range files {
		file := filepath.Join(d, f.Name())

		if f.Mode()&os.ModeTemporary != os.ModeTemporary {
			backupset[file] = struct{}{}

			if f.IsDir() && !excluded(file) {
				addfiles(file)
			}
		}
	}
}

func backup() {
	flag.Parse()

	log.Printf("Starting backup: name=%q schedule=%q\n", name, schedule)

	directories = make(map[string]bool)
	backupset = make(map[string]struct{})
	devices := make(map[string]bool)

	if mtab, err := mntent.Parse("/etc/mtab"); err != nil {
		log.Println("Failed to parse /etc/mtab: ", err)
	} else {
		for i := range mtab {
			if !devices[mtab[i].Name] && includeorexclude(mtab[i]) {
				devices[mtab[i].Name] = true
			}
		}
	}

	for d := range directories {
		if directories[d] {
			addfiles(d)
		}
	}

	//for f := range backupset {
	//fmt.Println(f)
	//}

	cmd := remotecommand("newbackup", "-name", name, "-schedule", schedule)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Println(scanner.Text()) // Println will add back the final '\n'
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}

func opencatalog() error {
	if db, err := sql.Open("sqlite3", filepath.Join(cfg.Catalog, "catalog.db")); err == nil {
		catalog = db

		if _, err = catalog.Exec(`
CREATE TABLE IF NOT EXISTS backups(name TEXT NOT NULL, schedule TEXT NOT NULL, date INTEGER PRIMARY KEY, finished INTEGER);
CREATE TABLE IF NOT EXISTS files(name TEXT NOT NULL, backupid INTEGER NOT NULL, hash TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS vault(hash TEXT PRIMARY KEY, size INTEGER);
			`); err != nil {
			return err
		}

		return nil
	} else {
		return err
	}
}

func newbackup() {
	flag.Parse()

	if sshclient := strings.Split(os.Getenv("SSH_CLIENT"), " "); sshclient[0] != "" {
		log.Printf("Remote client: ip=%q\n", sshclient[0])
	}

	if err := opencatalog(); err != nil {
		log.Fatal(err)
	}

	date = time.Now().Unix()
	for try := 0; try < 3; try++ {
		if _,err := catalog.Exec("INSERT INTO backups (date,name,schedule) VALUES(?,?,?)", date, name, schedule); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
		date = time.Now().Unix()
	}

	log.Printf("Creating backup set: date=%d name=%q schedule=%q\n", date, name, schedule)
	fmt.Println(date)
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

func loadconfig() {
	if _, err := toml.DecodeFile(defaultConfig, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse configuration: ", err)
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
	if logwriter, err := syslog.New(syslog.LOG_NOTICE, filepath.Base(os.Args[0])); err == nil {
		log.SetOutput(logwriter)
		log.SetFlags(0) // no need to add timestamp, syslog will do it for us
	}

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
		fmt.Fprintf(os.Stderr, "Usage: %s help [command]", programName)
	case "-help", "--help", "-h":
		fmt.Fprintln(os.Stderr, "Usage: %s COMMAND [options]\n\nCommands:", programName)
		fmt.Fprintln(os.Stderr, "  backup")
		fmt.Fprintln(os.Stderr, "  newbackup")
		fmt.Fprintln(os.Stderr, "  expire")
		fmt.Fprintln(os.Stderr, "  help")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\nTry '--help' for more information.\n", os.Args[0])
		os.Exit(1)
	}
}
