package main

import (
	"database/sql"
	"errors"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

const schemaVersion = 1

var catalog *sql.DB

func opencatalog() error {
	if err := os.MkdirAll(cfg.Vault, 0700); err != nil {
		return err
	}

	if db, err := sql.Open("sqlite3", cfg.Catalog); err == nil {
		catalog = db

		catalog.Exec("PRAGMA synchronous = OFF")
		catalog.Exec("PRAGMA journal_mode=WAL")
		catalog.Exec("PRAGMA busy_timeout = 3600000") // 1 hour timeout

		if _, err = catalog.Exec(`
CREATE TABLE IF NOT EXISTS META(name TEXT COLLATE NOCASE PRIMARY KEY, value TEXT);
INSERT OR IGNORE INTO META VALUES('schema', 1);
INSERT OR IGNORE INTO META VALUES('application', 'org.ezix.pukcab');
CREATE TABLE IF NOT EXISTS backups(name TEXT NOT NULL,
			schedule TEXT NOT NULL,
			date INTEGER PRIMARY KEY,
			finished INTEGER,
			files INTEGER,
			size INTEGER);
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
			devmajor INTEGER NOT NULL DEFAULT 0,
			devminor INTEGER NOT NULL DEFAULT 0,
			UNIQUE (backupid, name));
CREATE TRIGGER IF NOT EXISTS cleanup_files AFTER DELETE ON backups FOR EACH ROW
BEGIN
			DELETE FROM files WHERE backupid=OLD.date;
END;
			`); err != nil {
			return err
		}

		var schema string
		if err := catalog.QueryRow("SELECT value FROM META WHERE name='schema'").Scan(&schema); err == nil {
			if v, err := strconv.Atoi(schema); err != nil || v > schemaVersion {
				return errors.New("Unsupported catalog version, please upgrade")
			}
		} else {
			return err
		}
		return nil
	} else {
		return err
	}
}
