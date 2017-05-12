package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	flag.StringVar(&schedule, "schedule", "", "IGNORED") // kept for compatibility with older clients
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
	if err := syscall.Statfs(cfg.Vault, &fsstat); err == nil {
		if 10*fsstat.Bavail < fsstat.Blocks {
			log.Printf("Low disk space: msg=\"vault filling up (>90%%)\" available=%d required=%d where=%q error=warn\n", int64(fsstat.Bsize)*int64(fsstat.Bavail), int64(fsstat.Bsize)*int64(fsstat.Blocks)/10, absolute(cfg.Vault))
		}
	}

	// Check if we already have a backup running for this client
	if backups := Backups(repository, name, "*"); len(backups) > 0 {
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
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		f, err := strconv.Unquote(scanner.Text())
		if err != nil {
			f = scanner.Text()
		}
		f = path.Clean(f)
		manifest[metaname(f)] = git.File(empty)
	}

	// report new backup ID
	fmt.Println(date)

	if !full {
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

	// Find the most recent complete backup for this client
	if previous := Last(Finished(Backups(repository, name, "*"))); !previous.Finished.IsZero() {
		fmt.Println(previous.Date)
	} else {
		fmt.Println(0) // no previous backup
	}
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

	filter := flag.Args()
	if len(filter) == 0 {
		filter = append(filter, "*")
	}
	//namefilter := ConvertGlob("names.name", depth, filter...)

	tw := tar.NewWriter(os.Stdout)
	defer tw.Close()

	backups := Backups(repository, name, schedule)
	if date != 0 {
		details = true
		if what&Reverse != 0 {
			backups = After(date, backups)
			if len(backups) > 1 && what&SingleBackup != 0 {
				backups = []Backup{First(backups)}
			}
		} else {
			backups = Before(date, backups)
			if len(backups) > 1 && what&SingleBackup != 0 {
				backups = []Backup{Last(backups)}
			}
		}
	}

	for _, backup := range backups {
		var header bytes.Buffer
		if what&Data == 0 {
			enc := gob.NewEncoder(&header)
			enc.Encode(BackupInfo{
				Date:         backup.Date,
				Finished:     backup.Finished,
				LastModified: backup.LastModified,
				Name:         backup.Name,
				Schedule:     backup.Schedule,
				Files:        backup.Files,
				Size:         backup.Size,
			})

			globalhdr := &tar.Header{
				Name:     backup.Name,
				Linkname: backup.Schedule,
				ModTime:  time.Unix(int64(backup.Date), 0),
				Uid:      int(backup.Finished.Unix()),
				Typeflag: tar.TypeXGlobalHeader,
				Size:     int64(header.Len()),
			}
			tw.WriteHeader(globalhdr)
			tw.Write(header.Bytes())
		}

		if details {
			if ref := repository.Reference(backup.Date.String()); git.Valid(ref) {
				if err := repository.Recurse(ref, func(path string, node git.Node) error {
					if ismeta(path) {
						if obj, err := repository.Object(node); err == nil {
							if blob, ok := obj.(git.Blob); ok { //only consider blobs
								if r, err := blob.Open(); err == nil {
									if decoder := json.NewDecoder(r); decoder != nil {
										var meta Meta
										if err := decoder.Decode(&meta); err == nil || blob.Size() == 0 { // don't complain about empty metadata
											meta.Path = realname(path)
											hdr := meta.TarHeader()
											if what&Data == 0 {
												hdr.Size = 0
												if hdr.Typeflag == tar.TypeReg {
													if hdr.Xattrs == nil {
														hdr.Xattrs = make(map[string]string)
													}
													hdr.Xattrs["backup.size"] = fmt.Sprintf("%d", meta.Size)
													if data, err := repository.Get(ref, dataname(realname(path))); err == nil {
														hdr.Xattrs["backup.hash"] = string(data.ID())
													}
												}
											} else {
												if hdr.Typeflag == tar.TypeReg {
													hdr.Linkname = string(node.ID())
												}
											}
											tw.WriteHeader(hdr)
											if what&Data != 0 && hdr.Size > 0 {
												if data, err := repository.Get(ref, dataname(realname(path))); err == nil {
													if blob, ok := data.(git.Blob); ok {
														if reader, err := blob.Open(); err == nil {
															io.Copy(tw, reader)
															reader.Close()
														}
													}
												} else {
													failure.Println("Missing data from vault:", meta.Path, err)
												}
											}
										} else {
											failure.Println(err)
										}
									}
								} else {
									failure.Println(err)
								}
							}
						} else {
							failure.Println(err)
						}
					}
					return nil
				}); err != nil {
					LogExit(err)
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
	flag.StringVar(&schedule, "schedule", "", "Backup schedule")
	flag.StringVar(&schedule, "r", "", "-schedule")

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

	started := time.Now()

	backups := Backups(repository, name, "*")
	if Get(date, backups).Date == 0 { // we couldn't find this backup
		date = Last(backups).Date
	}
	if !Get(date, backups).Finished.IsZero() {
		failure.Printf("Error: backup set date=%d is already complete\n", date)
		log.Fatalf("Error: backup set date=%d is already complete\n", date)
	}

	files, missing := countfiles(repository, date)
	schedule = reschedule(date, name, schedule)

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
				manifest[dataname(hdr.Name)] = git.File(blob)

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

	files, missing = countfiles(repository, date)

	if missing == 0 { // the backup is complete, tag it
		repository.UnTag(date.String())
		repository.NewTag(date.String(), commit.ID(), commit.Type(), git.BlameMe(),
			JSON(BackupMeta{
				Date:     date,
				Name:     name,
				Schedule: schedule,
				Files:    files,
				Size:     received,
				Finished: time.Now().Unix(),
				// note: LastModified is 0
			}))

		log.Printf("Finished backup: date=%d name=%q schedule=%q files=%d received=%d duration=%.0f elapsed=%.0f\n", date, name, schedule, files, received, time.Since(started).Seconds(), time.Since(time.Unix(int64(date), 0)).Seconds())
		fmt.Printf("Backup %d complete (%d files)\n", date, files)
	} else {
		log.Printf("Received files for backup set: date=%d name=%q schedule=%q files=%d missing=%d received=%d duration=%.0f\n", date, name, schedule, files, missing, received, time.Since(started).Seconds())
		fmt.Printf("Received %d files for backup %d (%d files to go)\n", files-missing, date, missing)
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

	for _, backup := range Backups(repository, name, "*") {
		if date == -1 || backup.Date == date {
			if err := repository.UnTag(backup.Date.String()); err != nil {
				failure.Printf("Error: could not delete backup set date=%d\n", backup.Date)
				log.Printf("Deleting backup: date=%d name=%q error=warn msg=%q\n", backup.Date, backup.Name, err)
			} else {
				log.Printf("Deleted backup: date=%d name=%q\n", backup.Date, backup.Name)
			}
		}
	}
}

func vacuum() {
	// TODO
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
		expdate := date
		if date == -1 {
			switch schedule {
			case "daily":
				expdate = BackupID(time.Now().Unix() - days(cfg.Expiration.Daily, 2*7)*24*60*60) // 2 weeks
			case "weekly":
				expdate = BackupID(time.Now().Unix() - days(cfg.Expiration.Weekly, 6*7)*24*60*60) // 6 weeks
			case "monthly":
				expdate = BackupID(time.Now().Unix() - days(cfg.Expiration.Monthly, 365)*24*60*60) // 1 year
			case "yearly":
				expdate = BackupID(time.Now().Unix() - days(cfg.Expiration.Yearly, 10*365)*24*60*60) // 10 years
			default:
				failure.Println("Missing expiration")
				log.Fatal("Client did not provide an expiration")
			}
		}

		log.Printf("Expiring backups: name=%q schedule=%q date=%d (%v)\n", name, schedule, expdate, time.Unix(int64(expdate), 0))
		backups := Backups(repository, name, schedule)
		for i, backup := range backups {
			if i < len(backups)-keep && backup.Date < expdate {
				if err := repository.UnTag(backup.Date.String()); err != nil {
					failure.Printf("Error: could not delete backup set date=%d\n", backup.Date)
					log.Printf("Deleting backup: date=%d name=%q error=warn msg=%q\n", backup.Date, backup.Name, err)
				} else {
					log.Printf("Deleted backup: date=%d name=%q\n", backup.Date, backup.Name)
				}
			}
		}
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

	// TODO
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
	// TODO
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
