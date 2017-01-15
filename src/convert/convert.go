package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"github.com/boltdb/bolt"
	_ "github.com/lyonel/go-sqlite3"
	"log"
	"meta"
	"path"
	"time"

	"pukcab/tar"
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

func Store(root *bolt.Bucket, name string, mf *meta.File) error {
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
	catalogdb := "catalog.db"
	metadata := "META"

	flag.IntVar(&max, "max", 0, "Maximum number of backups to convert")
	flag.StringVar(&catalogdb, "i", "catalog.db", "Input file")
	flag.StringVar(&metadata, "o", "META", "Output file")

	flag.Parse()

	sqldb, err := sql.Open("sqlite3", catalogdb)
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	catalog := meta.New(metadata)
	catalog.SetTimeout(10 * time.Second)
	if info, err := catalog.Info(); err == nil {
		if info.Schema != 0 {
			log.Println("Schema version:", info.Schema)
		}
		if info.Application != "" {
			log.Println("Application:", info.Application)
		}
	} else {
		log.Fatal("Error: ", err)
	}

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

		err = catalog.Update(func(tx *meta.Tx) error {
			log.Printf("Processing backup date=%d name=%s schedule=%s files=%d\n", date.Int64, name.String, schedule.String, files.Int64)
			backupset, err := tx.CreateBackup(date.Int64)
			if err != nil {
				return err
			}
			backupset.Name = name.String
			backupset.Schedule = schedule.String
			backupset.Finished = finished.Int64
			backupset.Lastmodified = lastmodified.Int64
			backupset.Files = files.Int64
			backupset.Size = size.Int64
			err = backupset.Save()
			if err != nil {
				return err
			}

			sqlfiles, err := sqldb.Query("SELECT hash, type, names.name AS name, linknames.name AS linkname, size, birth, access, modify, change, mode, uid, gid, username, groupname, devmajor, devminor FROM files,names,names AS linknames WHERE backupid=? AND names.id=nameid AND linknames.id=linknameid", backupset.Date)
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
				file := &meta.File{
					Path:     name.String,
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
				if err := backupset.AddFile(file); err != nil {
					return err
				} else {
					nfiles++
					tsize += size.Int64
				}
			}
			if err := sqlfiles.Err(); err != nil {
				return err
			}

			if nfiles != backupset.Files {
				log.Printf("Warning files=%d expected=%d\n", nfiles, backupset.Files)
			}
			if tsize != backupset.Size {
				log.Printf("Warning size=%d expected=%d\n", tsize, backupset.Size)
			}

			return nil
		})

		if err != nil {
			log.Println("Skipped:", err)
			err = nil
		} else {
			log.Printf("Done files=%d size=%d\n", nfiles, tsize)
			count++
			time.Sleep(time.Second)
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
