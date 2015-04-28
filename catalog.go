package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/mattn/go-sqlite3"
)

const schemaVersion = 2

var catalog *sql.DB

type Catalog interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func opencatalog() error {
	if err := os.MkdirAll(cfg.Vault, 0700); err != nil {
		return err
	}

	if db, err := sql.Open("sqlite3", cfg.Catalog); err == nil {
		catalog = db

		catalog.Exec("PRAGMA synchronous = OFF")
		catalog.Exec("PRAGMA journal_mode=WAL")
		catalog.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d", timeout*1000))

		if _, err = catalog.Exec(`
CREATE TABLE IF NOT EXISTS META(name TEXT COLLATE NOCASE PRIMARY KEY, value TEXT);
INSERT OR IGNORE INTO META VALUES('schema', ?);
INSERT OR IGNORE INTO META VALUES('application', 'org.ezix.pukcab');
CREATE TABLE IF NOT EXISTS names(id INTEGER PRIMARY KEY, name TEXT, UNIQUE(name));
CREATE TABLE IF NOT EXISTS backups(name TEXT NOT NULL,
			schedule TEXT NOT NULL,
			date INTEGER PRIMARY KEY,
			finished INTEGER,
			files INTEGER,
			size INTEGER);
CREATE TABLE IF NOT EXISTS files(backupid INTEGER NOT NULL,
			hash TEXT COLLATE NOCASE NOT NULL DEFAULT '',
			type CHAR(1) NOT NULL DEFAULT '?',
			nameid INTEGER NOT NULL DEFAULT 0,
			linknameid INTEGER NOT NULL DEFAULT 0,
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
			devmajor INTEGER NOT NULL DEFAULT 0,
			devminor INTEGER NOT NULL DEFAULT 0,
			UNIQUE (backupid, nameid));
CREATE TRIGGER IF NOT EXISTS cleanup_files AFTER DELETE ON backups FOR EACH ROW
BEGIN
			DELETE FROM files WHERE backupid=OLD.date;
END;
			`, schemaVersion); err != nil {
			return err
		}

		var schema string
		if err := catalog.QueryRow("SELECT value FROM META WHERE name='schema'").Scan(&schema); err == nil {
			if v, err := strconv.Atoi(schema); err != nil || v > schemaVersion {
				return errors.New("Unsupported catalog version, please upgrade")
			} else {
				if v == 1 {
					log.Println("Upgrading catalog")
					tx, _ := catalog.Begin()
					if _, err := tx.Exec(`
ALTER TABLE files RENAME to oldfiles;
CREATE TABLE files(backupid INTEGER NOT NULL,
                        hash TEXT COLLATE NOCASE NOT NULL DEFAULT '',
                        type CHAR(1) NOT NULL DEFAULT '?',
                        nameid INTEGER NOT NULL DEFAULT 0,
                        linknameid INTEGER NOT NULL DEFAULT 0,
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
                        devmajor INTEGER NOT NULL DEFAULT 0,
                        devminor INTEGER NOT NULL DEFAULT 0,
                        UNIQUE (backupid, nameid));
INSERT OR IGNORE INTO names (name) SELECT name FROM oldfiles;
INSERT OR IGNORE INTO names (name) SELECT linkname FROM oldfiles;
INSERT INTO files (backupid,hash,type,nameid,linknameid,size,birth,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor) SELECT backupid,hash,type,names.id AS nameid,linknames.id AS linknameid,size,birth,access,modify,change,mode,uid,gid,username,groupname,devmajor,devminor FROM oldfiles,names,names AS linknames WHERE names.name=oldfiles.name AND linknames.name=oldfiles.linkname;
DROP TABLE oldfiles;
UPDATE META SET value=2 WHERE name='schema';
					`); err != nil {
						tx.Rollback()
						fmt.Fprintln(os.Stderr, "Catalog error")
						log.Fatal(err)
					}
					tx.Commit()
					v = 2
					log.Println("Upgraded catalog to version", v)
				}
			}
		} else {
			return err
		}
		return nil
	} else {
		return err
	}
}

func nameid(c Catalog, s string) (id int64) {
	if result, err := c.Exec("INSERT INTO names(name) VALUES(?)", s); err == nil {
		id, _ = result.LastInsertId()
	} else {
		err = c.QueryRow("SELECT id FROM names WHERE name=?", s).Scan(&id)
	}

	return id
}

func last(c Catalog, name string, schedule string) (id BackupID) {
	var date SQLInt
	c.QueryRow("SELECT MAX(date) FROM backups WHERE finished AND ? IN ('', name) AND ? IN ('', schedule)", name, schedule).Scan(&date)
	return BackupID(date)
}

func reschedule(backup BackupID, name string, s string) (schedule string) {
	if s != "auto" && s != "" {
		return s
	}

	schedule = defaultSchedule
	if schedule != "daily" { // only daily backups get re-scheduled
		return
	}

	latest, lastweekly, lastmonthly := last(catalog, name, ""),
		last(catalog, name, "weekly"),
		last(catalog, name, "monthly")

	if latest == 0 { // this is our first backup ever
		return
	}

	switch {
	case lastmonthly != 0 && backup-lastmonthly > 365*24*60*60:
		return "yearly"
	case lastweekly != 0 && backup-lastweekly > 30*24*60*60:
		return "monthly"
	case lastweekly == 0 || backup-lastweekly > 7*24*60*60:
		return "weekly"
	}

	return
}

func dberror(err error) (ok bool) {
	_, ok = err.(sqlite3.Error)
	return
}

func busy(err error) bool {
	if e, ok := err.(sqlite3.Error); ok {
		return e.Code == sqlite3.ErrBusy
	}
	return false
}
