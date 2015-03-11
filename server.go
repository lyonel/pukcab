package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"database/sql"
	"encoding/gob"
	"ezix.org/tar"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-sqlite3"
)

func SetupServer() {
	Setup()
	switchuser()
	if sshclient := strings.Split(os.Getenv("SSH_CLIENT"), " "); sshclient[0] != "" {
		log.Printf("Remote client: ip=%q\n", sshclient[0])
	}
}

func newbackup() {
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
	flag.BoolVar(&full, "full", full, "Full backup")
	flag.BoolVar(&full, "f", full, "-full")

	SetupServer()
	cfg.ServerOnly()

	if name == "" {
		fmt.Println(0)
		fmt.Fprintln(os.Stderr, "Missing backup name")
		log.Fatal("Client did not provide a backup name")
	}

	if schedule == "" {
		fmt.Println(0)
		fmt.Fprintln(os.Stderr, "Missing backup schedule")
		log.Fatal("Client did not provide a backup schedule")
	}

	if err := opencatalog(); err != nil {
		fmt.Println(0)
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	date = BackupID(time.Now().Unix())
	for try := 0; try < 3; try++ {
		if _, err := catalog.Exec("INSERT INTO backups (date,name,schedule) VALUES(?,?,?)", date, name, schedule); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
		date = BackupID(time.Now().Unix())
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
		f, err := strconv.Unquote(scanner.Text())
		if err != nil {
			f = scanner.Text()
		}
		if _, err := tx.Exec("INSERT INTO files (backupid,nameid) VALUES(?,?)", date, nameid(tx, filepath.Clean(f))); err != nil {
			tx.Rollback()
			fmt.Fprintln(os.Stderr, "Catalog error")
			log.Fatal(err)
		}
	}
	tx.Commit()

	fmt.Println(date)
	var previous SQLInt
	if err := catalog.QueryRow("SELECT MAX(date) AS previous FROM backups WHERE finished AND name=?", name).Scan(&previous); err == nil {
		if !full {
			_, err = catalog.Exec("WITH previous AS (SELECT * FROM files WHERE backupid=? AND nameid IN (SELECT nameid FROM files WHERE backupid=?)) INSERT OR REPLACE INTO files (backupid,hash,type,nameid,linknameid,size,birth,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor) SELECT ?,hash,type,nameid,linknameid,size,birth,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor FROM previous", previous, date, date)
		}
		if err == nil {
			fmt.Println(int64(previous))
		} else {
			fmt.Println(0) // no previous backup
		}
	} else {
		fmt.Println(0) // no previous backup
	}
}

func dumpcatalog(includedata bool) {
	details := false
	date = 0

	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	SetupServer()
	cfg.ServerOnly()

	if err := opencatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	filter := flag.Args()
	if len(filter) == 0 {
		filter = append(filter, "*")
	}
	for i, f := range filter {
		filter[i] = ConvertGlob(f)
	}

	tw := tar.NewWriter(os.Stdout)
	defer tw.Close()

	var stmt *sql.Stmt
	var err error
	if date != 0 {
		stmt, err = catalog.Prepare("SELECT date, name, schedule, finished, files, size FROM backups WHERE date<=? AND ? IN ('', name) ORDER BY date DESC LIMIT 1")
		details = true
	} else {
		if name != "" {
			stmt, err = catalog.Prepare("SELECT date, name, schedule, finished, files, size FROM backups WHERE ? NOT NULL AND name=? ORDER BY date")
		} else {
			stmt, err = catalog.Prepare("SELECT date, name, schedule, finished, files, size FROM backups WHERE ? NOT NULL AND ? NOT NULL ORDER BY date")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	if backups, err := stmt.Query(date, name); err == nil {
		defer backups.Close()
		for backups.Next() {
			var finished SQLInt
			var d SQLInt
			var f SQLInt
			var s SQLInt

			if err := backups.Scan(&d,
				&name,
				&schedule,
				&finished,
				&f,
				&s,
			); err != nil {
				fmt.Fprintln(os.Stderr, "Catalog error")
				log.Fatal(err)
			}

			date = BackupID(d)

			var header bytes.Buffer
			if !includedata {
				enc := gob.NewEncoder(&header)
				enc.Encode(BackupInfo{
					Date:     date,
					Finished: time.Unix(int64(finished), 0),
					Name:     name,
					Schedule: schedule,
					Files:    int64(f),
					Size:     int64(s),
				})

				globalhdr := &tar.Header{
					Name:     name,
					Linkname: schedule,
					ModTime:  time.Unix(int64(date), 0),
					Uid:      int(finished),
					Typeflag: tar.TypeXGlobalHeader,
					Size:     int64(header.Len()),
				}
				tw.WriteHeader(globalhdr)
				tw.Write(header.Bytes())
			}

			if details {
				if files, err := catalog.Query("SELECT names.name AS name,type,hash,links.name AS linkname,size,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor FROM files,names,names AS links WHERE backupid=? AND nameid=names.id AND linknameid=links.id ORDER BY name", int64(date)); err == nil {
					defer files.Close()
					for files.Next() {
						var hdr tar.Header
						var size int64
						var access int64
						var modify int64
						var change int64
						var hash string
						var filetype string
						var devmajor int64
						var devminor int64

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
							&devmajor,
							&devminor,
						); err == nil {
							hdr.Typeflag = '?'
							hdr.ModTime = time.Unix(modify, 0)
							hdr.Devmajor = devmajor
							hdr.Devminor = devminor
							hdr.AccessTime = time.Unix(access, 0)
							hdr.ChangeTime = time.Unix(change, 0)
							if filetype == string(tar.TypeReg) || filetype == string(tar.TypeRegA) {
								hdr.Typeflag = tar.TypeReg
								if includedata {
									hdr.Linkname = hash
								} else {
									hdr.Xattrs = make(map[string]string)
									hdr.Xattrs["backup.size"] = fmt.Sprintf("%d", size)
									if hash != "" {
										hdr.Xattrs["backup.hash"] = hash
									}
								}
							} else {
								if len(filetype) > 0 {
									hdr.Typeflag = filetype[0]
								}
							}
							for _, f := range filter {
								if matched, err := regexp.MatchString(f, hdr.Name); err == nil && matched {
									if includedata && hdr.Typeflag == tar.TypeReg {
										hdr.Size = size
									}
									tw.WriteHeader(&hdr)

									if includedata && size > 0 && hash != "" {
										if zdata, err := os.Open(filepath.Join(cfg.Vault, hash)); err == nil {
											gz, _ := gzip.NewReader(zdata)
											io.Copy(tw, gz)
											zdata.Close()
										} else {
											log.Println(err)
										}
									}
								}
							}
						} else {
							log.Println(err)
						}
					}
				} else {
					log.Println(err)
				}
			}
		}
	}
}

func metadata() {
	dumpcatalog(false)
}

func data() {
	dumpcatalog(true)
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
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	SetupServer()
	cfg.ServerOnly()

	if name == "" {
		fmt.Fprintln(os.Stderr, "Missing backup name")
		log.Fatal("Client did not provide a backup name")
	}

	if IsATTY(os.Stdout) {
		fmt.Fprintln(os.Stderr, "Should not be called directly")
		log.Fatal("Should not be called directly")
	}

	if err := opencatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	files := 0
	catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=?", date).Scan(&files)

	if files == 0 {
		var lastdate SQLInt
		catalog.QueryRow("SELECT MAX(date) FROM backups WHERE name=? AND schedule=?", name, schedule).Scan(&lastdate)
		date = BackupID(lastdate)
	}

	files = 0
	catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=?", date).Scan(&files)
	missing := 0
	catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=? AND type='?'", date).Scan(&missing)

	var finished SQLInt
	catalog.QueryRow("SELECT name,schedule,finished FROM backups WHERE date=?", date).Scan(&name, &schedule, &finished)

	if finished != 0 {
		fmt.Fprintf(os.Stderr, "Error: backup set date=%d is already complete\n", date)
		log.Fatalf("Error: backup set date=%d is already complete\n", date)
	}
	log.Printf("Receiving files for backup set: date=%d name=%q schedule=%q files=%d missing=%d\n", date, name, schedule, files, missing)

	tr := tar.NewReader(os.Stdin)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Catalog error")
			log.Fatal(err)
		}

		// skip fake entries used only for extended attributes and various metadata
		if hdr.Name != hdr.Linkname && hdr.Typeflag != tar.TypeXHeader && hdr.Typeflag != tar.TypeXGlobalHeader {
			var hash string
			checksum := sha512.New()

			if !filepath.IsAbs(hdr.Name) {
				hdr.Name = filepath.Join(string(filepath.Separator), hdr.Name)
			}

			if hdr.ModTime.IsZero() {
				hdr.ModTime = time.Unix(0, 0)
			}
			if hdr.AccessTime.IsZero() {
				hdr.AccessTime = time.Unix(0, 0)
			}
			if hdr.ChangeTime.IsZero() {
				hdr.ChangeTime = time.Unix(0, 0)
			}

			switch hdr.Typeflag {
			case tar.TypeReg, tar.TypeRegA:
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
						fmt.Fprintln(os.Stderr, "Catalog error")
						log.Fatal(err)
					}

					hash = EncodeHash(checksum.Sum(nil))

					if _, err := os.Stat(filepath.Join(cfg.Vault, hash)); os.IsNotExist(err) {
						os.Rename(tmpfile.Name(), filepath.Join(cfg.Vault, hash))
					} else {
						os.Remove(tmpfile.Name())
					}
				}

			}
			if stmt, err := catalog.Prepare("INSERT OR REPLACE INTO files (hash,backupid,nameid,size,type,linknameid,username,groupname,uid,gid,mode,access,modify,change,devmajor,devminor) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"); err != nil {
				fmt.Fprintln(os.Stderr, "Catalog error")
				log.Fatal(err)
			} else {
				for try := 0; try < cfg.Maxtries; try++ {
					_, err = stmt.Exec(hash, date, nameid(catalog, filepath.Clean(hdr.Name)), hdr.Size, string(hdr.Typeflag), nameid(catalog, filepath.Clean(hdr.Linkname)), hdr.Uname, hdr.Gname, hdr.Uid, hdr.Gid, hdr.Mode, hdr.AccessTime.Unix(), hdr.ModTime.Unix(), hdr.ChangeTime.Unix(), hdr.Devmajor, hdr.Devminor)
					if e, retry := err.(sqlite3.Error); retry {
						if e.Code != sqlite3.ErrBusy {
							break
						}
						log.Println(err, "- retrying", try+1)
						time.Sleep(time.Duration(1+rand.Intn(10)) * time.Second)
					} else {
						break
					}
				}
				if err != nil {
					fmt.Fprintln(os.Stderr, "Catalog error")
					log.Fatal(err)
				}
			}
		}
	}

	missing = 0
	if err := catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=? AND type='?'", date).Scan(&missing); err == nil {
		if missing == 0 {
			catalog.Exec("DELETE FROM files WHERE backupid=? AND type='X'", date)
			catalog.Exec("UPDATE backups SET finished=? WHERE date=?", time.Now().Unix(), date)
			catalog.Exec("UPDATE backups SET files=(SELECT COUNT(*) FROM files WHERE backupid=date) WHERE date=?", date)
			catalog.Exec("UPDATE backups SET size=(SELECT SUM(size) FROM files WHERE backupid=date) WHERE date=?", date)
			log.Printf("Finished backup: date=%d name=%q schedule=%q files=%d\n", date, name, schedule, files)
			fmt.Printf("Backup %d complete (%d files)\n", date, files)
		} else {
			log.Printf("Received files for backup set: date=%d name=%q schedule=%q files=%d missing=%d\n", date, name, schedule, files, missing)
			fmt.Printf("Received %d files for backup %d (%d files to go)\n", files-missing, date, missing)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}
}

func purgebackup() {
	date = 0
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	SetupServer()
	cfg.ServerOnly()

	if name == "" {
		fmt.Fprintln(os.Stderr, "Missing backup name")
		log.Fatal("Client did not provide a backup name")
	}

	if date == 0 {
		fmt.Fprintln(os.Stderr, "Missing backup date")
		log.Fatal("Client did not provide a backup date")
	}

	if err := opencatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	if r, err := catalog.Exec("DELETE FROM backups WHERE date=? AND name=?", date, name); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	} else {
		if n, _ := r.RowsAffected(); n < 1 {
			fmt.Println("Backup not found.")
			return
		} else {
			log.Printf("Deleted backup: date=%d name=%q\n", date, name)
		}
	}
}

func vacuum() {
	if tx, err := catalog.Begin(); err == nil {
		defer tx.Commit()
		log.Println("Vacuum...")

		tx.Exec("DELETE FROM files WHERE backupid NOT IN (SELECT date FROM backups)")
		tx.Exec("VACUUM")

		unused := make(map[string]struct{})
		if vaultfiles, err := ioutil.ReadDir(cfg.Vault); err == nil {
			for _, f := range vaultfiles {
				if time.Since(f.ModTime()).Hours() > 24 { // f is older than 24 hours
					unused[f.Name()] = struct{}{}
				}
			}
		} else {
			log.Println(err)
			return
		}

		if datafiles, err := tx.Query("SELECT DISTINCT hash FROM files"); err == nil {
			defer datafiles.Close()
			for datafiles.Next() {
				var f string
				if err := datafiles.Scan(&f); err == nil {
					delete(unused, f)
				} else {
					log.Println(err)
					return
				}
			}
		} else {
			log.Println(err)
			return
		}

		for f := range unused {
			if err := os.Remove(filepath.Join(cfg.Vault, f)); err != nil {
				log.Println(err)
				return
			}
		}

		log.Printf("Vacuum: removed %d files\n", len(unused))
	} else {
		log.Println(err)
		return
	}
}

func expirebackup() {
	keep := 3
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
	flag.IntVar(&keep, "keep", keep, "Minimum number of backups to keep")
	flag.IntVar(&keep, "k", keep, "-keep")
	flag.Var(&date, "age", "Maximum age/date")
	flag.Var(&date, "a", "-age")
	flag.Var(&date, "date", "-age")
	flag.Var(&date, "d", "-age")

	SetupServer()
	cfg.ServerOnly()

	if schedule == "" {
		fmt.Fprintln(os.Stderr, "Missing backup schedule")
		log.Fatal("Client did not provide a backup schedule")
	}

	if date == -1 {
		switch schedule {
		case "daily":
			date = BackupID(time.Now().Unix() - 14*24*60*60) // 2 weeks
		case "weekly":
			date = BackupID(time.Now().Unix() - 42*24*60*60) // 6 weeks
		case "monthly":
			date = BackupID(time.Now().Unix() - 365*24*60*60) // 1 year
		case "yearly":
			date = BackupID(time.Now().Unix() - 10*365*24*60*60) // 10 years
		default:
			fmt.Fprintln(os.Stderr, "Missing expiration")
			log.Fatal("Client did not provide an expiration")
		}
	}

	if err := opencatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	log.Printf("Expiring backups: name=%q schedule=%q date=%d (%v)\n", name, schedule, date, time.Unix(int64(date), 0))

	tx, _ := catalog.Begin()
	if _, err := tx.Exec("DELETE FROM backups WHERE date<? AND ? IN (name,'') AND schedule=?", date, name, schedule); err != nil {
		tx.Rollback()
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}
	tx.Commit()

	vacuum()
}

func printstats(name string, stat *syscall.Statfs_t) {
	fmt.Printf("%-10s\t%s\t%s\t%s\t%.0f%%\t%s\n", Fstype(uint64(stat.Type)), Bytes(uint64(stat.Bsize)*stat.Blocks), Bytes(uint64(stat.Bsize)*(stat.Blocks-stat.Bavail)), Bytes(uint64(stat.Bsize)*stat.Bavail), 100-100*float64(stat.Bavail)/float64(stat.Blocks), name)
}

func df() {
	SetupServer()
	cfg.ServerOnly()

	if err := opencatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	var cstat, vstat syscall.Statfs_t
	if err := syscall.Statfs(cfg.Catalog, &cstat); err != nil {
		log.Fatal(err)
	}
	if err := syscall.Statfs(cfg.Vault, &vstat); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Filesystem\tSize\tUsed\tAvail\tUse%\tMounted on")
	if cstat.Fsid == vstat.Fsid {
		printstats("catalog,vault", &cstat)
	} else {
		printstats("catalog", &cstat)
		printstats("vault", &vstat)
	}

	var backups, names, schedules, files, size SQLInt
	if err := catalog.QueryRow("SELECT COUNT(*),COUNT(DISTINCT name),COUNT(DISTINCT schedule),SUM(files),SUM(size) FROM backups").Scan(&backups, &names, &schedules, &files, &size); err == nil {
		fmt.Println()
		fmt.Println("Backup names:", names)
		fmt.Println("Retention schedules:", schedules)
		fmt.Println("Backup sets:", backups)
		fmt.Printf("Data in vault: %s (%d files)\n", Bytes(uint64(size)), files)
		fmt.Printf("Compression factor: %.1f\n", float64(size)/(float64(vstat.Bsize)*float64(vstat.Blocks-vstat.Bavail)))
	}
}

func dbmaintenance() {
	SetupServer()
	cfg.ServerOnly()

	if err := opencatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Catalog error")
		log.Fatal(err)
	}

	vacuum()
}
