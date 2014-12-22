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
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var catalog *sql.DB

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
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
	flag.BoolVar(&full, "full", full, "Full backup")
	flag.BoolVar(&full, "f", full, "-full")
	flag.Parse()

	if sshclient := strings.Split(os.Getenv("SSH_CLIENT"), " "); sshclient[0] != "" {
		log.Printf("Remote client: ip=%q\n", sshclient[0])
	}

	if name == "" {
		fmt.Println(0)
		fmt.Println("Missing backup name")
		log.Fatal("Client did not provide a backup name")
	}

	if schedule == "" {
		fmt.Println(0)
		fmt.Println("Missing backup schedule")
		log.Fatal("Client did not provide a backup schedule")
	}

	if err := opencatalog(); err != nil {
		fmt.Println(0)
		fmt.Println(err)
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

	//if err != nil {
	//fmt.Println(0)
	//fmt.Println(err)
	//log.Fatal(err)
	//}

	log.Printf("Creating backup set: date=%d name=%q schedule=%q\n", date, name, schedule)

	// Now, get ready to receive file list
	tx, _ := catalog.Begin()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		scanner.Text()
		if _, err := tx.Exec("INSERT INTO files (backupid,name) VALUES(?,?)", date, filepath.Clean(scanner.Text())); err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	tx.Commit()

	fmt.Println(date)
	var previous SQLInt
	if err := catalog.QueryRow("SELECT MAX(date) AS previous FROM backups WHERE finished AND name=?", name).Scan(&previous); err == nil {
		fmt.Println(int64(previous))
	} else {
		fmt.Println(0) // no previous backup
	}
}

func backupinfo() {
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Int64Var(&date, "date", 0, "Backup set")
	flag.Int64Var(&date, "d", 0, "-date")
	flag.Parse()

	if err := opencatalog(); err != nil {
		log.Fatal(err)
	}

	tw := tar.NewWriter(os.Stdout)
	defer tw.Close()

	globaldata := paxHeaders(map[string]interface{}{
		".name":     name,
		".schedule": schedule,
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

	if files, err := catalog.Query("SELECT name,type,hash,linkname,size,access,modify,change,mode,uid,gid,username,groupname FROM files WHERE backupid=? ORDER BY name", int64(date)); err == nil {
		defer files.Close()
		for files.Next() {
			var hdr tar.Header
			var size int64
			var access int64
			var modify int64
			var change int64
			var hash string
			var filetype string

			if err := files.Scan(&hdr.Name,
				&filetype,
				&hash,
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
				hdr.Xattrs = make(map[string]string)
				hdr.Xattrs["backup.type"] = filetype
				if hash != "" {
					hdr.Xattrs["backup.hash"] = hash
				}
				if size > 0 {
					hdr.Xattrs["backup.size"] = fmt.Sprintf("%d", size)
				}
				hdr.Typeflag = 'Z'
				tw.WriteHeader(&hdr)
			} else {
				log.Println(err)
			}
		}
	} else {
		log.Println(err)
	}
}

func toascii(s string) (result string) {
	for i := 0; i < len(s); i++ {
		if s[i] > ' ' && s[i] < 0x80 {
			result += string(s[i])
		}
	}
	return
}

func submitfiles() {
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Int64Var(&date, "date", date, "Backup set")
	flag.Int64Var(&date, "d", date, "-date")
	flag.Parse()

	if sshclient := strings.Split(os.Getenv("SSH_CLIENT"), " "); sshclient[0] != "" {
		log.Printf("Remote client: ip=%q\n", sshclient[0])
	}

	if name == "" {
		fmt.Println("Missing backup name")
		log.Fatal("Client did not provide a backup name")
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

			if !filepath.IsAbs(hdr.Name) {
				hdr.Name = filepath.Join(string(filepath.Separator), hdr.Name)
			}

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
				if tmpfile, err := ioutil.TempFile(cfg.Vault, programName+"-"); err == nil {
					gz := gzip.NewWriter(tmpfile)
					gz.Header.Name = toascii(filepath.Base(hdr.Name))
					gz.Header.ModTime = hdr.ModTime
					gz.Header.OS = gzipOS
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

					os.Rename(tmpfile.Name(), filepath.Join(cfg.Vault, hash))
				}

			}
			if _, err := catalog.Exec("INSERT OR REPLACE INTO files (hash,backupid,name,size,type,linkname,username,groupname,uid,gid,mode,access,modify,change) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)", hash, date, filepath.Clean(hdr.Name), hdr.Size, string(hdr.Typeflag), filepath.Clean(hdr.Linkname), hdr.Uname, hdr.Gname, hdr.Uid, hdr.Gid, hdr.Mode, hdr.AccessTime.Unix(), hdr.ModTime.Unix(), hdr.ChangeTime.Unix()); err != nil {
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
