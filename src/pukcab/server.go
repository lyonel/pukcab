package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	//"crypto/sha512"
	"database/sql"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ezix.org/src/pkg/git"
	"pukcab/tar"
)

type DumpFlags int

const (
	Short DumpFlags = 1 << iota
	FullDetails
	SingleBackup
	Reverse
	Data
)

func SetupServer() {
	Setup()

	if cfg.Web != "" {
		web := remotecommand("web")
		web.Stdin = nil
		web.Stdout = nil
		web.Stderr = nil

		os.Setenv("PUKCAB_WEB", "auto")
		web.Start()
		go web.Wait()
	}
	switchuser()
	failure.SetPrefix("")
	if sshclient := strings.Split(os.Getenv("SSH_CLIENT"), " "); sshclient[0] != "" {
		log.Printf("Remote client: ip=%q\n", sshclient[0])
	}
}

func absolute(s string) string {
	if result, err := filepath.Abs(s); err == nil {
		return result
	} else {
		return s
	}
}

func newbackup() {
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.StringVar(&schedule, "schedule", "", "Backup schedule")
	flag.StringVar(&schedule, "r", "", "-schedule")
	flag.BoolVar(&full, "full", full, "Full backup")
	flag.BoolVar(&full, "f", full, "-full")

	SetupServer()
	cfg.ServerOnly()

	if name == "" {
		fmt.Println(0)
		failure.Println("Missing backup name")
		log.Fatal("Client did not provide a backup name")
	}

	if err := opencatalog(); err != nil {
		fmt.Println(0)
		LogExit(err)
	}

	var fsstat syscall.Statfs_t
	if err := syscall.Statfs(cfg.Catalog, &fsstat); err == nil {
		var page_size, page_count SQLInt
		catalog.QueryRow("PRAGMA page_count").Scan(&page_count)
		catalog.QueryRow("PRAGMA page_size").Scan(&page_size)

		if int64(page_size*page_count) > int64(fsstat.Bsize)*int64(fsstat.Bavail)/3 {
			log.Printf("Low disk space: msg=\"catalog disk filling up\" available=%d required=%d where=%q error=warn\n", int64(fsstat.Bsize)*int64(fsstat.Bavail), 3*page_size*page_count, absolute(cfg.Catalog))
		}
	}
	if err := syscall.Statfs(cfg.Vault, &fsstat); err == nil {
		if 10*fsstat.Bavail < fsstat.Blocks {
			log.Printf("Low disk space: msg=\"vault filling up (>90%%)\" available=%d required=%d where=%q error=warn\n", int64(fsstat.Bsize)*int64(fsstat.Bavail), int64(fsstat.Bsize)*int64(fsstat.Blocks)/10, absolute(cfg.Vault))
		}
	}

	// Check if we already have a backup running for this client
	if backups := Backups(name, "*"); len(backups) > 0 {
		for _, b := range backups {
			if time.Since(b.LastModified).Hours() < 1 && !force { // a backup was modified less than 1 hour ago
				failure.Println("Another backup is already running")
				LogExit(errors.New("Another backup is already running"))
			}
		}
	}

	// Generate and record a new backup ID
	if err := retry(cfg.Maxtries, func() error {
		date = BackupID(time.Now().Unix())
		schedule = reschedule(date, name, schedule)
		if git.Valid(repository.Reference(date.String())) { // this backup ID already exists
			return errors.New("Duplicate backup ID")
		}
		catalog.Exec("INSERT INTO backups (date,name,schedule,lastmodified,size) VALUES(?,?,?,?,0)", date, name, schedule, date)
		return repository.TagBranch(name, date.String())
	}); err != nil {
		LogExit(err)
	}

	log.Printf("Creating backup set: date=%d name=%q schedule=%q\n", date, name, schedule)

	// Now, get ready to receive file list
	empty, err := repository.NewEmptyBlob()
	if err != nil {
		LogExit(err)
	}
	manifest := git.Manifest{}
	tx, _ := catalog.Begin()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		f, err := strconv.Unquote(scanner.Text())
		if err != nil {
			f = scanner.Text()
		}
		f = path.Clean(f)
		manifest[metaname(f)] = git.File(empty)
		if _, err := tx.Exec("INSERT INTO files (backupid,nameid) VALUES(?,?)", date, nameid(tx, filepath.Clean(f))); err != nil {
			tx.Rollback()
			LogExit(err)
		}
	}
	tx.Exec("UPDATE backups SET lastmodified=? WHERE date=?", time.Now().Unix(), date)
	tx.Commit()

	// report new backup ID
	fmt.Println(date)

	if !full {
		catalog.Exec("WITH previousbackups AS (SELECT date FROM backups WHERE name=? AND date<? ORDER BY date DESC LIMIT 2), newfiles AS (SELECT nameid from files where backupid=?) INSERT OR REPLACE INTO files (backupid,hash,type,nameid,linknameid,size,birth,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor) SELECT ?,hash,type,nameid,linknameid,size,birth,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor FROM (SELECT * FROM files WHERE type!='?' AND nameid IN newfiles AND backupid IN previousbackups ORDER BY backupid) GROUP BY nameid", name, date, date, date)
		catalog.Exec("UPDATE backups SET lastmodified=? WHERE date=?", time.Now().Unix(), date)

		if previous := repository.Reference(name); git.Valid(previous) {
			repository.Recurse(previous, func(path string, node git.Node) error {
				if _, ok := manifest[metaname(realname(path))]; ok {
					manifest[path] = node
				}
				return nil
			})
		}
	}
	_, err = repository.CommitToBranch(name, manifest, git.BlameMe(), git.BlameMe(), "New backup\n")
	if err != nil {
		LogExit(err)
	}
	repository.TagBranch(name, date.String())

	// Check if we have a complete backup for this client
	if backups := Backups(name, "*"); len(backups) > 0 {
		for i := len(backups) - 1; i >= 0; i-- {
			if !backups[i].Finished.IsZero() {
				fmt.Println(backups[i].Finished.Unix())
				return
			}
		}
	}
	fmt.Println(0) // no previous backup
}

func dumpcatalog(what DumpFlags) {
	details := what&FullDetails != 0
	date = 0

	depth := infinite

	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.StringVar(&schedule, "schedule", "", "Backup schedule")
	flag.StringVar(&schedule, "r", "", "-schedule")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.IntVar(&depth, "depth", infinite, "Descent depth")

	SetupServer()
	cfg.ServerOnly()

	if err := opencatalog(); err != nil {
		LogExit(err)
	}
	autocheckpoint(catalog, false)

	filter := flag.Args()
	if len(filter) == 0 {
		filter = append(filter, "*")
	}
	namefilter := ConvertGlob("names.name", depth, filter...)

	tw := tar.NewWriter(os.Stdout)
	defer tw.Close()

	var query string
	var stmt *sql.Stmt
	var err error
	if date != 0 {
		query = "SELECT date, name, schedule, finished, lastmodified, files, size FROM backups WHERE date"
		if what&Reverse != 0 {
			query += ">="
		} else {
			query += "<="
		}
		query += "? AND ? IN ('', name) AND ? IN ('', schedule) ORDER BY date"
		if what&Reverse == 0 {
			query += " DESC"
		}
		if what&SingleBackup != 0 {
			query += " LIMIT 1"
		}
		details = true
	} else {
		query = "SELECT date, name, schedule, finished, lastmodified, files, size FROM backups WHERE ? NOT NULL AND ? IN ('', name) AND ? IN ('', schedule) ORDER BY date"
	}

	stmt, err = catalog.Prepare(query)
	if err != nil {
		LogExit(err)
	}

	if backups, err := stmt.Query(date, name, schedule); err == nil {
		defer backups.Close()
		for backups.Next() {
			var finished SQLInt
			var lastmodified SQLInt
			var d SQLInt
			var f SQLInt
			var s SQLInt

			if err := backups.Scan(&d,
				&name,
				&schedule,
				&finished,
				&lastmodified,
				&f,
				&s,
			); err != nil {
				LogExit(err)
			}

			date = BackupID(d)

			var header bytes.Buffer
			if what&Data == 0 {
				enc := gob.NewEncoder(&header)
				enc.Encode(BackupInfo{
					Date:         date,
					Finished:     time.Unix(int64(finished), 0),
					LastModified: time.Unix(int64(lastmodified), 0),
					Name:         name,
					Schedule:     schedule,
					Files:        int64(f),
					Size:         int64(s),
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
				if files, err := catalog.Query("SELECT names.name AS name,type,hash,links.name AS linkname,size,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor FROM files,names,names AS links WHERE backupid=? AND nameid=names.id AND linknameid=links.id AND ("+namefilter+") ORDER BY name", int64(date)); err == nil {
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
								if what&Data != 0 {
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
							if what&Data != 0 && hdr.Typeflag != tar.TypeSymlink && hdr.Typeflag != tar.TypeLink {
								hdr.Linkname = ""
							}
							if what&Data != 0 && hdr.Typeflag == tar.TypeReg {
								hdr.Size = size
							}
							if hdr.Typeflag == tar.TypeReg && !Exists(filepath.Join(cfg.Vault, hash)) {
								log.Printf("Vault corrupted: msg=\"data file missing\" vault=%q hash=%q name=%q date=%d file=%q error=critical\n", absolute(cfg.Vault), hash, name, date, hdr.Name)
								failure.Println("Missing from vault:", hdr.Name)
							} else {
								tw.WriteHeader(&hdr)

								if what&Data != 0 && size > 0 && hash != "" {
									if zdata, err := os.Open(filepath.Join(cfg.Vault, hash)); err == nil {
										gz, _ := gzip.NewReader(zdata)
										io.Copy(tw, gz)
										zdata.Close()
									} else {
										log.Println(err)
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
	dumpcatalog(SingleBackup)
}

func data() {
	dumpcatalog(Data | SingleBackup)
}

func timeline() {
	dumpcatalog(FullDetails | Reverse)
}

func toascii(s string) (result string) {
	for i := 0; i < len(s); i++ {
		if s[i] > ' ' && s[i] < 0x80 {
			result += string(s[i])
		}
	}
	return
}

type TarReader struct {
	tar.Reader
	size int64
}

func (tr *TarReader) Size() (int64, error) {
	return tr.size, nil
}

func submitfiles() {
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	SetupServer()
	cfg.ServerOnly()

	if name == "" {
		failure.Println("Missing backup name")
		log.Fatal("Client did not provide a backup name")
	}

	if IsATTY(os.Stdout) {
		failure.Println("Should not be called directly")
		log.Fatal("Should not be called directly")
	}

	if err := opencatalog(); err != nil {
		LogExit(err)
	}

	files := 0
	started := time.Now()
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
		failure.Printf("Error: backup set date=%d is already complete\n", date)
		log.Fatalf("Error: backup set date=%d is already complete\n", date)
	}
	log.Printf("Receiving files for backup set: date=%d name=%q schedule=%q files=%d missing=%d\n", date, name, schedule, files, missing)

	manifest := git.Manifest{}
	var received int64
	tr := tar.NewReader(os.Stdin)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			LogExit(err)
		}

		// skip fake entries used only for extended attributes and various metadata
		if hdr.Name != hdr.Linkname && hdr.Typeflag != tar.TypeXHeader && hdr.Typeflag != tar.TypeXGlobalHeader {
			var hash string
			//checksum := sha512.New()

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

			if meta, err := repository.NewBlob(bytes.NewReader([]byte(JSON(HeaderMeta(hdr))))); err == nil {
				manifest[metaname(hdr.Name)] = git.File(meta)
			} else {
				LogExit(err)
			}

			catalog.Exec("UPDATE backups SET lastmodified=? WHERE date=?", time.Now().Unix(), date)

			switch hdr.Typeflag {
			case tar.TypeReg, tar.TypeRegA:
				blob, err := repository.NewBlob(&TarReader{
					Reader: *tr,
					size:   hdr.Size,
				})
				if err != nil {
					LogExit(err)
				}
				received += hdr.Size
				hash = string(blob.ID())
				manifest[dataname(hdr.Name)] = git.File(blob)
				/*
					if tmpfile, err := ioutil.TempFile(cfg.Vault, programName+"-"); err == nil {
						gz := gzip.NewWriter(tmpfile)
						gz.Header.Name = toascii(filepath.Base(hdr.Name))
						gz.Header.ModTime = hdr.ModTime
						gz.Header.OS = gzipOS
						buf := make([]byte, 1024*1024) // 1MiB
						for {
							nr, er := tr.Read(buf)
							catalog.Exec("UPDATE backups SET lastmodified=?,size=size+? WHERE date=?", time.Now().Unix(), nr, date)
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
							LogExit(err)
						}

						received += hdr.Size
						hash = EncodeHash(checksum.Sum(nil))

						if _, err := os.Stat(filepath.Join(cfg.Vault, hash)); os.IsNotExist(err) {
							os.Rename(tmpfile.Name(), filepath.Join(cfg.Vault, hash))
						} else {
							os.Remove(tmpfile.Name())
							os.Chtimes(filepath.Join(cfg.Vault, hash), time.Now(), time.Now())
						}
					}*/

			}
			if stmt, err := catalog.Prepare("INSERT OR REPLACE INTO files (hash,backupid,nameid,size,type,linknameid,username,groupname,uid,gid,mode,access,modify,change,devmajor,devminor) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"); err != nil {
				LogExit(err)
			} else {
				if err := retryif(cfg.Maxtries, busy, func() error {
					_, err := stmt.Exec(hash, date, nameid(catalog, filepath.Clean(hdr.Name)), hdr.Size, string(hdr.Typeflag), nameid(catalog, filepath.Clean(hdr.Linkname)), hdr.Uname, hdr.Gname, hdr.Uid, hdr.Gid, hdr.Mode, hdr.AccessTime.Unix(), hdr.ModTime.Unix(), hdr.ChangeTime.Unix(), hdr.Devmajor, hdr.Devminor)
					return err
				}); err != nil {
					LogExit(err)
				}
			}
		}
	}

	if previous := repository.Reference(name); git.Valid(previous) {
		repository.Recurse(previous, func(path string, node git.Node) error {
			if _, defined := manifest[path]; !defined {
				manifest[path] = node
			}
			return nil
		})
	}
	commit, err := repository.CommitToBranch(name, manifest, git.BlameMe(), git.BlameMe(), "Submit files\n")
	if err != nil {
		LogExit(err)
	}
	repository.TagBranch(name, date.String())

	catalog.Exec("UPDATE backups SET lastmodified=NULL WHERE date=?", date)
	checkpoint(catalog, false)

	missing = 0
	if err := catalog.QueryRow("SELECT COUNT(*) FROM files WHERE backupid=? AND type='?'", date).Scan(&missing); err == nil {
		if missing == 0 { // the backup is complete, tag it
			repository.UnTag(date.String())
			repository.NewTag(date.String(), commit.ID(), commit.Type(), git.BlameMe(),
				JSON(BackupMeta{
					Date:     date,
					Name:     name,
					Schedule: schedule,
					Files:    int64(files),
					Size:     received,
					Finished: time.Now().Unix(),
					// note: LastModified is 0
				}))

			catalog.Exec("DELETE FROM files WHERE backupid=? AND type='X'", date)
			catalog.Exec("UPDATE backups SET finished=? WHERE date=?", time.Now().Unix(), date)
			catalog.Exec("UPDATE backups SET files=(SELECT COUNT(*) FROM files WHERE backupid=date) WHERE date=?", date)
			catalog.Exec("UPDATE backups SET size=(SELECT SUM(size) FROM files WHERE backupid=date) WHERE date=?", date)
			log.Printf("Finished backup: date=%d name=%q schedule=%q files=%d received=%d duration=%.0f elapsed=%.0f\n", date, name, schedule, files, received, time.Since(started).Seconds(), time.Since(time.Unix(int64(date), 0)).Seconds())
			fmt.Printf("Backup %d complete (%d files)\n", date, files)
		} else {
			log.Printf("Received files for backup set: date=%d name=%q schedule=%q files=%d missing=%d received=%d duration=%.0f\n", date, name, schedule, files, missing, received, time.Since(started).Seconds())
			fmt.Printf("Received %d files for backup %d (%d files to go)\n", files-missing, date, missing)
		}
	} else {
		LogExit(err)
	}
}

func purgebackup() {
	date = -1
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	SetupServer()
	cfg.ServerOnly()

	if name == "" {
		failure.Println("Missing backup name")
		log.Fatal("Client did not provide a backup name")
	}

	if date == -1 && !force {
		failure.Println("Missing backup date")
		log.Fatal("Client did not provide a backup date")
	}

	if err := opencatalog(); err != nil {
		LogExit(err)
	}

	if r, err := catalog.Exec("DELETE FROM backups WHERE ? IN (date,-1) AND name=?", date, name); err != nil {
		LogExit(err)
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
	checkpoint(catalog, true)

	done := make(chan error)
	go func() {
		done <- backupcatalog()
	}()

	used := make(map[string]bool)
	if datafiles, err := catalog.Query("SELECT DISTINCT hash FROM files"); err == nil {
		defer datafiles.Close()
		for datafiles.Next() {
			var f string
			if err := datafiles.Scan(&f); err == nil {
				used[f] = true
			} else {
				log.Println(err)
				return
			}
		}
	} else {
		log.Println(err)
		return
	}

	var unused, kept, freed, remaining int64

	if vaultfiles, err := ioutil.ReadDir(cfg.Vault); err == nil {
		for _, f := range vaultfiles {
			if time.Since(f.ModTime()).Hours() > 24 && !used[f.Name()] { // f is older than 24 hours
				unused++
				if err := os.Remove(filepath.Join(cfg.Vault, f.Name())); err != nil {
					log.Println(err)
					return
				} else {
					freed += f.Size()
				}
			} else {
				kept++
				remaining += f.Size()
			}
		}
	} else {
		log.Println(err)
		return
	}

	log.Printf("Vacuum: removed=%d kept=%d freed=%d used=%d\n", unused, kept, freed, remaining)

	if err := <-done; err != nil {
		log.Printf("Could not backup catalog: msg=%q error=warn\n", err)
	}
}

func days(val, def int64) int64 {
	if val > 0 {
		return val
	} else {
		return def
	}
}

func expirebackup() {
	schedules := ""
	keep := 3
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.StringVar(&schedules, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedules, "r", defaultSchedule, "-schedule")
	flag.IntVar(&keep, "keep", keep, "Minimum number of backups to keep")
	flag.IntVar(&keep, "k", keep, "-keep")
	flag.Var(&date, "age", "Maximum age/date")
	flag.Var(&date, "a", "-age")
	flag.Var(&date, "date", "-age")
	flag.Var(&date, "d", "-age")

	SetupServer()
	cfg.ServerOnly()

	if schedules == "" {
		failure.Println("Missing backup schedule")
		log.Fatal("Client did not provide a backup schedule")
	}

	if err := opencatalog(); err != nil {
		LogExit(err)
	}

	for _, schedule = range strings.Split(schedules, ",") {

		if date == -1 {
			switch schedule {
			case "daily":
				date = BackupID(time.Now().Unix() - days(cfg.Expiration.Daily, 2*7)*24*60*60) // 2 weeks
			case "weekly":
				date = BackupID(time.Now().Unix() - days(cfg.Expiration.Weekly, 6*7)*24*60*60) // 6 weeks
			case "monthly":
				date = BackupID(time.Now().Unix() - days(cfg.Expiration.Monthly, 365)*24*60*60) // 1 year
			case "yearly":
				date = BackupID(time.Now().Unix() - days(cfg.Expiration.Yearly, 10*365)*24*60*60) // 10 years
			default:
				failure.Println("Missing expiration")
				log.Fatal("Client did not provide an expiration")
			}
		}

		log.Printf("Expiring backups: name=%q schedule=%q date=%d (%v)\n", name, schedule, date, time.Unix(int64(date), 0))

		tx, _ := catalog.Begin()
		if _, err := tx.Exec(fmt.Sprintf("CREATE TEMPORARY VIEW expendable AS SELECT backups.date FROM backups WHERE backups.finished IS NOT NULL AND backups.date NOT IN (SELECT date FROM backups AS sets WHERE backups.name=sets.name ORDER BY date DESC LIMIT %d)", keep)); err != nil {
			tx.Rollback()
			LogExit(err)
		}

		if _, err := tx.Exec("DELETE FROM backups WHERE date<? AND ? IN (name,'') AND schedule=? AND date IN (SELECT * FROM expendable)", date, name, schedule); err != nil {
			tx.Rollback()
			LogExit(err)
		}
		tx.Exec("DROP VIEW expendable")
		tx.Commit()
	}

	vacuum()
}

func printstats(name string, stat *syscall.Statfs_t) {
	fmt.Printf("%-10s\t%s\t%s\t%s\t%.0f%%\t%s\n", Fstype(uint64(stat.Type)), Bytes(uint64(stat.Bsize)*stat.Blocks), Bytes(uint64(stat.Bsize)*(stat.Blocks-stat.Bavail)), Bytes(uint64(stat.Bsize)*stat.Bavail), 100-100*float32(stat.Bavail)/float32(stat.Blocks), name)
}

func df() {
	SetupServer()
	cfg.ServerOnly()

	if err := opencatalog(); err != nil {
		LogExit(err)
	}

	var cstat, vstat syscall.Statfs_t
	if err := syscall.Statfs(cfg.Catalog, &cstat); err != nil {
		LogExit(err)
	}
	if err := syscall.Statfs(cfg.Vault, &vstat); err != nil {
		LogExit(err)
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
		fmt.Printf("Compression factor: %.1f\n", float32(size)/(float32(vstat.Bsize)*float32(vstat.Blocks-vstat.Bavail)))
	}
}

func dbmaintenance() {
	SetupServer()
	cfg.ServerOnly()

	if err := opencatalog(); err != nil {
		LogExit(err)
	}

	vacuum()
}

func fsck(fix bool) {
	errors := 0

	if tx, err := catalog.Begin(); err == nil {
		info.Println("#1: [catalog] checking integrity")
		result, err := tx.Exec("PRAGMA integrity_check")
		if err != nil {
			tx.Rollback()
			LogExit(err)
		}

		info.Println("#2: [catalog] checking orphan files")
		if fix {
			result, err = tx.Exec("DELETE FROM files WHERE backupid NOT IN (SELECT date FROM backups)")
			if err != nil {
				tx.Rollback()
				LogExit(err)
			}
			if n, _ := result.RowsAffected(); n > 0 {
				fmt.Printf("%d orphan files deleted\n", n)
				log.Printf("Catalog fix: orphans=%d\n", n)
			}
		} else {
			var n SQLInt
			err = tx.QueryRow("SELECT COUNT(*) FROM files WHERE backupid NOT IN (SELECT date FROM backups)").Scan(&n)
			if err != nil {
				tx.Rollback()
				LogExit(err)
			}
			if n > 0 {
				fmt.Printf("%d orphan files\n", n)
				log.Printf("Catalog check: orphans=%d\n", n)
			}
			errors += int(n)
		}

		info.Println("#3: [catalog] checking nameless files")
		if fix {
			result, err = tx.Exec("INSERT OR IGNORE INTO names SELECT nameid,'/lost+found/'||nameid FROM files WHERE nameid NOT IN (SELECT id FROM names)")
			if err != nil {
				tx.Rollback()
				LogExit(err)
			}
			if n, _ := result.RowsAffected(); n > 0 {
				fmt.Printf("%d file names recovered\n", n)
				log.Printf("Catalog fix: foundfiles=%d\n", n)
			}
		} else {
			var n SQLInt
			err = tx.QueryRow("SELECT COUNT(*) FROM files WHERE nameid NOT IN (SELECT id FROM names)").Scan(&n)
			if err != nil {
				tx.Rollback()
				LogExit(err)
			}
			if n > 0 {
				fmt.Printf("%d nameless files\n", n)
				log.Printf("Catalog check: lostfiles=%d\n", n)
			}
			errors += int(n)
		}

		info.Println("#4: [catalog] checking nameless links")
		if fix {
			result, err = tx.Exec("INSERT OR IGNORE INTO names SELECT linknameid,'/lost+found/'||linknameid FROM files WHERE linknameid NOT IN (SELECT id FROM names)")
			if err != nil {
				tx.Rollback()
				LogExit(err)
			}
			if n, _ := result.RowsAffected(); n > 0 {
				fmt.Printf("%d link names recovered\n", n)
				log.Printf("Catalog fix: foundlinks=%d\n", n)
			}
		} else {
			var n SQLInt
			err = tx.QueryRow("SELECT COUNT(*) FROM files WHERE linknameid NOT IN (SELECT id FROM names)").Scan(&n)
			if err != nil {
				tx.Rollback()
				LogExit(err)
			}
			if n > 0 {
				fmt.Printf("%d nameless links\n", n)
				log.Printf("Catalog check: lostlinks=%d\n", n)
			}
			errors += int(n)
		}

		info.Println("#5: [vault] checking data files")
		if datafiles, err := tx.Query("SELECT DISTINCT hash FROM files WHERE type IN (?,?)", tar.TypeReg, tar.TypeRegA); err == nil {
			n := 0
			defer datafiles.Close()
			for datafiles.Next() {
				var hash string
				if err := datafiles.Scan(&hash); err == nil {
					if hash != "" && !Exists(filepath.Join(cfg.Vault, hash)) {
						n++
						if fix {
							_, err = tx.Exec("UPDATE files SET type='?' WHERE hash=?", hash)
							if err != nil {
								tx.Rollback()
								LogExit(err)
							}
						}
					}
				} else {
					log.Println(err)
					return
				}
			}
			if n > 0 {
				fmt.Printf("%d missing datafiles\n", n)
				log.Printf("Vault check: lostfiles=%d\n", n)
			}
			errors += int(n)
		} else {
			log.Println(err)
			return
		}

		tx.Commit()
	} else {
		LogExit(err)
	}

	if !fix && errors > 0 {
		fmt.Println(errors, "errors found.")
		os.Exit(1)
	}
}

func dbcheck() {
	nofix := false

	flag.BoolVar(&nofix, "dontfix", nofix, "Don't fix issues")
	flag.BoolVar(&nofix, "nofix", nofix, "-dontfix")
	flag.BoolVar(&nofix, "N", nofix, "-dontfix")

	SetupServer()
	cfg.ServerOnly()

	if err := opencatalog(); err != nil {
		LogExit(err)
	}

	fsck(!nofix)
}
