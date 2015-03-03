package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/antage/mntent"
)

type Backup struct {
	Date           BackupID
	Name, Schedule string

	backupset   map[string]struct{}
	directories map[string]bool

	include, exclude, ignore []string
}

func NewBackup(cfg Config) *Backup {
	return &Backup{

		include: cfg.Include,
		exclude: cfg.Exclude,
		ignore:  []string{},
	}
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

	directories[e.Directory] = result
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

	if contains(b.exclude, d) {
		return
	}

	files, _ := ioutil.ReadDir(d)
	for _, f := range files {
		file := filepath.Join(d, f.Name())

		if !IsNodump(f, file) {
			b.backupset[file] = struct{}{}

			if f.IsDir() && !b.excluded(file) {
				b.addfiles(file)
			}
		}
	}
}

func (b *Backup) Start(name string, schedule string) {
	b.Name, b.Schedule = name, schedule

	backupset = make(map[string]struct{})
	directories = make(map[string]bool)
	devices := make(map[string]bool)

	if mtab, err := loadmtab(); err != nil {
		log.Println("Failed to parse /etc/mtab: ", err)
	} else {
		for _, m := range mtab {
			if !devices[m.Name] && includeorexclude(m) {
				devices[m.Name] = true
			}
		}
	}

	for _, i := range cfg.Include {
		if filepath.IsAbs(i) {
			directories[i] = true
		}
	}

	for d := range directories {
		if directories[d] {
			addfiles(d)
		}
	}

	for _, f := range b.ignore {
		delete(b.backupset, f)
	}
}

func (b *Backup) Ignore(files ...string) {
	b.ignore = append(b.ignore, files...)
}

func (b *Backup) Count() int {
	return len(b.backupset)
}

func (b *Backup) ForEach(action func(string)) {
	for f := range backupset {
		action(f)
	}
}
