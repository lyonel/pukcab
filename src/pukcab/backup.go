package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antage/mntent"

	"pukcab/tar"
)

// Backup represents a backup set
type Backup struct {
	Date           BackupID
	Name, Schedule string
	Started        time.Time
	Finished       time.Time
	LastModified   time.Time
	Files          int64
	Size           int64

	backupset   map[string]struct{}
	directories map[string]bool

	include, exclude, ignore []string
}

// NewBackup creates a new backup set (ignoring the catalog files on server)
func NewBackup(cfg Config) (backup *Backup) {
	backup = &Backup{
		include: cfg.Include,
		exclude: cfg.Exclude,
		ignore:  []string{},
	}

	if cfg.IsServer() {
		if pw, err := Getpwnam(cfg.User); err == nil {
			if filepath.IsAbs(cfg.Catalog) {
				backup.Ignore(cfg.Catalog,
					cfg.Catalog+"~",
					cfg.Catalog+"-shm",
					cfg.Catalog+"-wal")
			} else {
				backup.Ignore(filepath.Join(pw.Dir, cfg.Catalog),
					filepath.Join(pw.Dir, cfg.Catalog+"~"),
					filepath.Join(pw.Dir, cfg.Catalog+"-shm"),
					filepath.Join(pw.Dir, cfg.Catalog+"-wal"))
			}
		}
	}

	return
}

func contains(set []string, e string) bool {
	for _, a := range set {
		if a == e {
			return true
		}

		if filepath.IsAbs(a) {
			if matched, _ := filepath.Match(a, e); matched && strings.ContainsAny(a, "*?[") {
				return true
			}

			if strings.HasPrefix(e, a+string(filepath.Separator)) {
				return true
			}
		} else {
			if matched, _ := filepath.Match(a, filepath.Base(e)); matched && strings.ContainsAny(a, "*?[") {
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

func (b *Backup) includeorexclude(e *mntent.Entry) bool {
	result := !(contains(b.exclude, e.Types[0]) || contains(b.exclude, e.Directory)) && (contains(b.include, e.Types[0]) || contains(b.include, e.Directory))

	b.directories[e.Directory] = result
	return result
}

func (b *Backup) excluded(f string) bool {
	if _, known := b.directories[f]; known {
		return !b.directories[f]
	}
	return contains(b.exclude, f) && !contains(b.include, f)
}

func (b *Backup) addfiles(d string) {
	b.backupset[d] = struct{}{}
	debug.Println("Adding", d)

	if contains(b.exclude, d) {
		return
	}

	files, _ := ioutil.ReadDir(d)
	for _, f := range files {
		file := filepath.Join(d, f.Name())

		if !IsNodump(f, file) {
			b.backupset[file] = struct{}{}
			debug.Println("Adding", file)

			if f.IsDir() && !b.excluded(file) {
				b.addfiles(file)
			}
		}
	}
}

// Start starts a new backup and enumerates the files to be backed-up
func (b *Backup) Start(name string, schedule string) {
	b.Name, b.Schedule = name, schedule
	b.Started = time.Now()

	b.backupset = make(map[string]struct{})
	b.directories = make(map[string]bool)
	devices := make(map[string]bool)

	if mtab, err := loadmtab(); err != nil {
		log.Println("Failed to parse /etc/mtab: ", err)
	} else {
		for _, m := range mtab {
			if !devices[m.Name] && b.includeorexclude(m) {
				devices[m.Name] = true
			}
		}
	}

	for _, i := range cfg.Include {
		if filepath.IsAbs(i) {
			b.directories[i] = true
		}
	}

	for d := range b.directories {
		if b.directories[d] {
			b.addfiles(d)
		}
	}

	for _, f := range b.ignore {
		delete(b.backupset, f)
	}
}

// Init initialises from an existing backup
func (b *Backup) Init(date BackupID, name string) {
	b.Name, b.Date = name, date
	b.backupset = make(map[string]struct{})
}

// Ignore adds files/tree/mountpoint to the ignore list
func (b *Backup) Ignore(files ...string) {
	b.ignore = append(b.ignore, files...)
}

// Forget removes files from the backup set
func (b *Backup) Forget(files ...string) {
	for _, f := range files {
		delete(b.backupset, f)
	}
}

// Add includes files into the backup set
func (b *Backup) Add(files ...string) {
	for _, f := range files {
		b.backupset[f] = struct{}{}
	}
}

// Count returns the number of entries in the backup set
func (b *Backup) Count() int {
	return len(b.backupset)
}

// ForEach enumerates entries and performs action on each
func (b *Backup) ForEach(action func(string)) {
	for f := range b.backupset {
		action(f)
	}
}

// Status is the result of verifying a backup entry against the actual filesystem
type Status int

// used when checking a backup
const (
	OK = iota
	Modified
	MetaModified
	Deleted
	Missing
	Unknown
)

// Check verifies that a given backup entry (identified as tar record) has been changed
func Check(hdr tar.Header, quick bool) (result Status) {
	result = Unknown

	switch hdr.Typeflag {
	case '?':
		return Missing
	case tar.TypeXHeader, tar.TypeXGlobalHeader:
		return OK
	}

	if hdr.Name == "" {
		return Missing
	}

	if fi, err := os.Lstat(hdr.Name); err == nil {
		fhdr, err := tar.FileInfoHeader(fi, hdr.Linkname)
		if err == nil {
			fhdr.Uname = Username(fhdr.Uid)
			fhdr.Gname = Groupname(fhdr.Gid)
		} else {
			return
		}
		result = OK
		if fhdr.Mode != hdr.Mode ||
			fhdr.Uid != hdr.Uid ||
			fhdr.Gid != hdr.Gid ||
			fhdr.Uname != hdr.Uname ||
			fhdr.Gname != hdr.Gname ||
			!fhdr.ModTime.IsZero() && !hdr.ModTime.IsZero() && fhdr.ModTime.Unix() != hdr.ModTime.Unix() ||
			!fhdr.AccessTime.IsZero() && !hdr.AccessTime.IsZero() && fhdr.AccessTime.Unix() != hdr.AccessTime.Unix() ||
			!fhdr.ChangeTime.IsZero() && !hdr.ChangeTime.IsZero() && fhdr.ChangeTime.Unix() != hdr.ChangeTime.Unix() ||
			fhdr.Typeflag != hdr.Typeflag ||
			fhdr.Typeflag == tar.TypeSymlink && fhdr.Linkname != hdr.Linkname {
			result = MetaModified
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return
		}
		if hdr.Size != fhdr.Size {
			result = Modified
			return
		}

		if quick && result != OK {
			return
		}

		if hash, ok := hdr.Xattrs["backup.hash"]; ok {
			if hash1, hash2 := Hash(hdr.Name); hash != hash1 && hash != hash2 {
				result = Modified
			}
		}
	} else {
		if os.IsNotExist(err) {
			result = Deleted
		}
		return
	}

	return
}
