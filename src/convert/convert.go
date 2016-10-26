package main

import (
	"database/sql"
	"encoding/json"
	"ezix.org/tar"
	"flag"
	"github.com/boltdb/bolt"
	_ "github.com/lyonel/go-sqlite3"
	"log"
	"meta"
	"path"
)

func Encode(v interface{}) []byte {
	result, _ := json.Marshal(v)
	return result
}

func MkPath(root *bolt.Bucket, name string) (*bolt.Bucket, error) {
	dir, base := path.Split(name)
	if base == "" {
		return root, nil
	} else {
		parent, err := MkPath(root, path.Clean(dir))
		if err != nil {
			return nil, err
		}
		return parent.CreateBucketIfNotExists([]byte(base))
	}
}

func Store(root *bolt.Bucket, name string, mf *meta.FileMetadata) error {
	cwd, err := MkPath(root, name)
	if err != nil {
		return err
	}

	if err := cwd.Put([]byte("."), Encode(mf)); err != nil {
		return err
	}
	return nil
}

func main() {
	count := 0
	max := 3
	catalog := "catalog.db"
	metadata := "META"

	flag.IntVar(&max, "max", 0, "Maximum number of backups to convert")
	flag.StringVar(&catalog, "i", "catalog.db", "Input file")
	flag.StringVar(&metadata, "o", "META", "Output file")

	flag.Parse()

	sqldb, err := sql.Open("sqlite3", catalog)
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	db, err := bolt.Open(metadata, 0640, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlbackups, err := sqldb.Query("SELECT name, schedule, date, finished, lastmodified, files, size FROM backups ORDER BY date DESC")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlbackups.Close()

	for sqlbackups.Next() {
		var tsize, nfiles int64
		var (
			name, schedule                            sql.NullString
			date, finished, lastmodified, files, size sql.NullInt64
		)

		if err := sqlbackups.Scan(&name,
			&schedule,
			&date,
			&finished,
			&lastmodified,
			&files,
			&size); err != nil {
			log.Fatal(err)
		}

		err = db.Update(func(tx *bolt.Tx) error {
			backups, err := tx.CreateBucketIfNotExists([]byte("backups"))
			if err != nil {
				return err
			}
			backup := &meta.BackupMetadata{
				Name:         name.String,
				Schedule:     schedule.String,
				Date:         date.Int64,
				Finished:     finished.Int64,
				Lastmodified: lastmodified.Int64,
				Files:        files.Int64,
				Size:         size.Int64,
			}
			log.Printf("Processing backup date=%d name=%s schedule=%s files=%d\n", backup.Date, backup.Name, backup.Schedule, backup.Files)
			backupset, err := backups.CreateBucket(Encode(backup.Date))
			if err != nil {
				return err
			}
			fileset, err := backupset.CreateBucket([]byte("files"))
			if err != nil {
				return err
			}

			sqlfiles, err := sqldb.Query("SELECT hash, type, names.name AS name, linknames.name AS linkname, size, birth, access, modify, change, mode, uid, gid, username, groupname, devmajor, devminor FROM files,names,names AS linknames WHERE backupid=? AND names.id=nameid AND linknames.id=linknameid", backup.Date)
			if err != nil {
				return err
			}
			defer sqlfiles.Close()

			for sqlfiles.Next() {
				var (
					hash, filetype, name, linkname, username, groupname                     sql.NullString
					size, birth, access, modify, change, mode, uid, gid, devmajor, devminor sql.NullInt64
				)

				if err := sqlfiles.Scan(&hash,
					&filetype,
					&name,
					&linkname,
					&size,
					&birth,
					&access,
					&modify,
					&change,
					&mode,
					&uid,
					&gid,
					&username,
					&groupname,
					&devmajor,
					&devminor); err != nil {
					return err
				}
				file := &meta.FileMetadata{
					Hash:     hash.String,
					Type:     filetype.String,
					Target:   linkname.String,
					Size:     size.Int64,
					Created:  birth.Int64,
					Accessed: access.Int64,
					Modified: modify.Int64,
					Changed:  change.Int64,
					Mode:     mode.Int64,
					Uid:      uid.Int64,
					Gid:      gid.Int64,
					Owner:    username.String,
					Group:    groupname.String,
					DevMajor: devmajor.Int64,
					DevMinor: devminor.Int64,
				}
				if file.Type != string(tar.TypeSymlink) && file.Type != string(tar.TypeLink) {
					file.Target = ""
				}
				if err := Store(fileset, name.String, file); err != nil {
					return err
				} else {
					nfiles++
					tsize += size.Int64
				}
			}
			if err := sqlfiles.Err(); err != nil {
				return err
			}

			if nfiles != backup.Files {
				log.Printf("Warning files=%d expected=%d\n", nfiles, backup.Files)
			}
			if tsize != backup.Size {
				log.Printf("Warning size=%d expected=%d\n", tsize, backup.Size)
			}

			return backupset.Put([]byte("info"), Encode(backup))
		})
		if err != nil {
			log.Println("Skipped:", err)
			err = nil
		} else {
			log.Printf("Done files=%d size=%d\n", nfiles, tsize)
			count++
		}

		if max != 0 && count >= max {
			break
		}
	}
	if err := sqlbackups.Err(); err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}

}
