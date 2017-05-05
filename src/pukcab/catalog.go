package main

import (
	"database/sql"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ezix.org/src/pkg/git"
)

const schemaVersion = 3

var repository *git.Repository

type Catalog interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func opencatalog() error {
	if err := os.MkdirAll(cfg.Vault, 0700); err != nil {
		return err
	}

	if r, err := git.Create(filepath.Join(cfg.Vault, ".git")); err == nil {
		repository = r
		repository.Describe("Pukcab vault")
	} else {
		return err
	}

	return nil
}

func min(a, b BackupID) BackupID {
	if a > b {
		return b
	} else {
		return a
	}
}

func midnight(backup BackupID) BackupID {
	y, m, d := backup.Time().Date()
	return BackupID(time.Date(y, m, d, 0, 0, 0, 0, time.Local).Unix())
}

func reschedule(backup BackupID, name string, s string) (schedule string) {
	if s != "auto" && s != "" {
		return s
	}

	schedule = defaultSchedule
	if schedule != "daily" { // only daily backups get re-scheduled
		return
	}

	var earliest, firstweekly, firstmonthly, latest, lastweekly, lastmonthly, lastyearly BackupID

	for _, b := range Backups(repository, name, "*") {
		if earliest == 0 || b.Date < earliest {
			earliest = b.Date
		}
		if b.Date > latest {
			latest = b.Date
		}

		switch b.Schedule {
		case "weekly":
			if firstweekly == 0 || b.Date < firstweekly {
				firstweekly = b.Date
			}
			if b.Date > lastweekly {
				lastweekly = b.Date
			}
		case "monthly":
			if firstmonthly == 0 || b.Date < firstmonthly {
				firstmonthly = b.Date
			}
			if b.Date > lastmonthly {
				lastmonthly = b.Date
			}
		case "yearly":
			if b.Date > lastyearly {
				lastyearly = b.Date
			}
		}
	}

	if latest == 0 { // this is our first backup ever
		return
	}

	today := midnight(backup)

	switch {
	case firstmonthly != 0 && min(today-firstmonthly, today-lastyearly) > 365*24*60*60:
		return "yearly"
	case firstweekly != 0 && min(today-firstweekly, today-lastmonthly) > 31*24*60*60:
		return "monthly"
	case earliest != 0 && min(today-earliest, today-lastweekly) > 7*24*60*60:
		return "weekly"
	}

	return
}

func busy(err error) bool {
	return false
}

func metaname(p string) string {
	return path.Join("META", p, "...")
}

func dataname(p string) string {
	return path.Join("DATA", p)
}

func ismeta(path string) bool {
	return strings.HasPrefix(path, "META/") && strings.HasSuffix(path, "/...")
}

func realname(path string) string {
	if ismeta(path) {
		return strings.TrimPrefix(strings.TrimSuffix(path, "/..."), "META/")
	} else {
		return strings.TrimPrefix(path, "DATA/")
	}
}

func countfiles(repository *git.Repository, date BackupID) (files int64, missing int64) {
	empty, _ := repository.NewEmptyBlob()
	if ref := repository.Reference(date.String()); git.Valid(ref) {
		if err := repository.Recurse(repository.Reference(date.String()),
			func(name string, node git.Node) error {
				if ismeta(name) {
					files++
					if node.ID() == empty.ID() { // metadata is empty
						missing++
					}
				}
				return nil
			}); err != nil {
			LogExit(err)
		}
	}
	return
}
