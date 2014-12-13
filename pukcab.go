package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha512"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/antage/mntent"
	_ "github.com/mattn/go-sqlite3"
)

const programName = "pukcab"
const versionMajor = 1
const versionMinor = 0
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

var vault string = "/var/" + programName + "/vault"

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

	cmd := remotecommand("newbackup", "-name", name, "-schedule", schedule)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	for f := range backupset {
		fmt.Fprintln(stdin, f)
	}
	stdin.Close()

	tr := tar.NewReader(stdout)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		if hdr.Typeflag == 'E' {
			fmt.Println("Server error:", hdr.Name)
			log.Fatal("Server error:", hdr.Name)
		}

		fmt.Printf("%v\n", hdr.Name)
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			date = hdr.ModTime.Unix()
			log.Printf("New backup: date=%d files=%d\n", date, len(backupset))
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}

func opencatalog() error {
	if db, err := sql.Open("sqlite3", filepath.Join(cfg.Catalog, "catalog.db")); err == nil {
		catalog = db

		catalog.Exec("PRAGMA synchronous = OFF")

		if _, err = catalog.Exec(`
CREATE TABLE IF NOT EXISTS backups(name TEXT NOT NULL,
			schedule TEXT NOT NULL,
			date INTEGER PRIMARY KEY,
			finished INTEGER);
CREATE TABLE IF NOT EXISTS files(backupid INTEGER NOT NULL,
			hash TEXT COLLATE NOCASE NOT NULL DEFAULT '',
			type CHAR(1) NOT NULL DEFAULT '?',
			name TEXT NOT NULL,
			linkname TEXT NOT NULL DEFAULT '',
			size INTEGER NOT NULL DEFAULT -1,
			birth INTEGER NOT NULL DEFAULT 0,
			access INTEGER NOT NULL DEFAULT 0,
			modify INTEGER NOT NULL DEFAULT 0,
			change INTEGER NOT NULL DEFAULT 0,
			mode INTEGER NOT NULL DEFAULT 0,
			uid INTEGER NOT NULL DEFAULT 0,
			gid INTEGER NOT NULL DEFAULT 0,
			username TEXT NOT NULL DEFAULT '',
			groupname TEXT NOT NULL DEFAULT '',
			UNIQUE (backupid, name));
CREATE TABLE IF NOT EXISTS vault(hash TEXT COLLATE NOCASE PRIMARY KEY,
			size INTEGER NOT NULL DEFAULT -1);
CREATE TRIGGER IF NOT EXISTS cleanup_files AFTER DELETE ON backups FOR EACH ROW
BEGIN
			DELETE FROM files WHERE backupid=OLD.date;
END;
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
		if _, err := catalog.Exec("INSERT INTO backups (date,name,schedule) VALUES(?,?,?)", date, name, schedule); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
		date = time.Now().Unix()
	}

	log.Printf("Creating backup set: date=%d name=%q schedule=%q\n", date, name, schedule)

	tw := tar.NewWriter(os.Stdout)
	defer tw.Close()

	var previous SQLInt
	if err := catalog.QueryRow("SELECT MAX(date) AS previous FROM backups WHERE finished AND name=?", name).Scan(&previous); err == nil {
		globaldata := paxHeaders(map[string]interface{}{
			".name":     name,
			".schedule": schedule,
			".previous": int64(previous),
			".version":  fmt.Sprintf("%d.%d", versionMajor, versionMinor),
		})
		globalhdr := &tar.Header{
			Name:     name,
			Size:     int64(len(globaldata)),
			Linkname: schedule,
			ModTime:  time.Unix(date, 0),
			Typeflag: tar.TypeXGlobalHeader,
		}
		tw.WriteHeader(globalhdr)
		tw.Write(globaldata)

		if files, err := catalog.Query("SELECT name,hash,size,access,modify,change,mode,uid,gid,username,groupname FROM files WHERE backupid=? ORDER BY name", int64(previous)); err == nil {
			defer files.Close()
			for files.Next() {
				var hdr tar.Header
				var size int64
				var access int64
				var modify int64
				var change int64

				if err := files.Scan(&hdr.Name,
					&hdr.Linkname,
					&size,
					&access,
					&modify,
					&change,
					&hdr.Mode,
					&hdr.Uid,
					&hdr.Gid,
					&hdr.Uname,
					&hdr.Gname,
				); err == nil {
					hdr.ModTime = time.Unix(modify, 0)
					hdr.AccessTime = time.Unix(access, 0)
					hdr.ChangeTime = time.Unix(change, 0)
					if hdr.Linkname == "" {
						hdr.Typeflag = tar.TypeDir
					} else {
						hdr.Typeflag = tar.TypeSymlink
					}
					tw.WriteHeader(&hdr)
				} else {
					log.Println(err)
				}
			}
		} else {
			log.Println(err)
		}
	} else {
		globalhdr := &tar.Header{
			Name:     fmt.Sprintf("%v", err),
			ModTime:  time.Now(),
			Typeflag: 'E',
		}
		tw.WriteHeader(globalhdr)
		log.Println(err)
	}

	// Now, get ready to receive file list
	tx, _ := catalog.Begin()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		scanner.Text()
		if _, err := tx.Exec("INSERT INTO files (backupid,name) VALUES(?,?)", date, path.Clean(scanner.Text())); err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	tx.Commit()
}

func submitfiles() {
	flag.Int64Var(&date, "date", date, "Backup set")
	flag.Int64Var(&date, "d", date, "-date")
	flag.Parse()

	if sshclient := strings.Split(os.Getenv("SSH_CLIENT"), " "); sshclient[0] != "" {
		log.Printf("Remote client: ip=%q\n", sshclient[0])
	}

	if err := opencatalog(); err != nil {
		log.Fatal(err)
	}

	files := 0
	catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=?", date).Scan(&files)

	if files == 0 {
		var lastdate SQLInt
		catalog.QueryRow("SELECT MAX(date) FROM backups WHERE name=? AND schedule=?", name, schedule).Scan(&lastdate)
		date = int64(lastdate)
	}

	files = 0
	catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=?", date).Scan(&files)
	missing := 0
	catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=? AND type='?'", date).Scan(&missing)

	var finished SQLInt
	catalog.QueryRow("SELECT name,schedule,finished FROM backups WHERE date=?", date).Scan(&name, &schedule, &finished)

	log.Printf("Receiving files for backup set: date=%d name=%q schedule=%q files=%d missing=%d\n", date, name, schedule, files, missing)
	if finished != 0 {
		log.Printf("Warning: backup set date=%d is already complete\n", date)
	}

	tr := tar.NewReader(os.Stdin)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		// skip usually fake entries used only for extended attributes
		if hdr.Name != hdr.Linkname {
			var zero time.Time
			var hash string
			checksum := sha512.New()

			if hdr.ModTime == zero {
				hdr.ModTime = time.Unix(0, 0)
			}
			if hdr.AccessTime == zero {
				hdr.AccessTime = time.Unix(0, 0)
			}
			if hdr.ChangeTime == zero {
				hdr.ChangeTime = time.Unix(0, 0)
			}

			if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA {
				if tmpfile, err := ioutil.TempFile(vault, programName+"-"); err == nil {
					gz := gzip.NewWriter(tmpfile)
					gz.Header.Name = name + ":" + hdr.Name
					buf := make([]byte, 1024*1024) // 1MiB
					for {
						nr, er := tr.Read(buf)
						if nr > 0 {
							nw, ew := gz.Write(buf[0:nr])
							checksum.Write(buf[0:nr])
							if ew != nil {
								err = ew
								break
							}
							if nr != nw {
								err = io.ErrShortWrite
								break
							}
						}
						if er == io.EOF {
							break
						}
						if er != nil {
							err = er
							break
						}
					}
					gz.Close()
					tmpfile.Close()

					if err != nil {
						log.Fatal(err)
					}

					hash = fmt.Sprintf("%x", checksum.Sum(nil))

					os.Rename(tmpfile.Name(), vault+string(filepath.Separator)+hash)
				}

			}
			if _, err := catalog.Exec("INSERT OR REPLACE INTO files (hash,backupid,name,size,type,linkname,username,groupname,uid,gid,mode,access,modify,change) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)", hash, date, path.Clean(hdr.Name), hdr.Size, string(hdr.Typeflag), path.Clean(hdr.Linkname), hdr.Uname, hdr.Gname, hdr.Uid, hdr.Gid, hdr.Mode, hdr.AccessTime.Unix(), hdr.ModTime.Unix(), hdr.ChangeTime.Unix()); err != nil {
				log.Fatal(err)
			}
		}
	}

	missing = 0
	if err := catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=? AND type='?'", date).Scan(&missing); err == nil {
		if missing == 0 {
			catalog.Exec("UPDATE backups SET finished=? WHERE date=?", time.Now().Unix(), date)
			log.Printf("Finished backup: date=%d name=%q schedule=%q files=%d\n", date, name, schedule, files)
			fmt.Printf("Backup %d complete (%d files)\n", date, files)
		} else {
			log.Printf("Received files for backup set: date=%d name=%q schedule=%q files=%d missing=%d\n", date, name, schedule, files, missing)
			fmt.Printf("Received %d files for backup %d (%d files to go)\n", files-missing, date, missing)
		}
	} else {
		log.Fatal(err)
	}
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
