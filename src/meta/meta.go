package meta

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"path"
)

type Catalog struct {
	db   *bolt.DB
	path string
}

type Fileset *bolt.Bucket

type Backup struct {
	Name         string `json:"name,omitempty"`
	Schedule     string `json:"schedule,omitempty"`
	Date         int64  `json:"date,omitempty"`
	Finished     int64  `json:"finished,omitempty"`
	Files        int64  `json:"files,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Lastmodified int64  `json:"lastmodified,omitempty"`
}

type File struct {
	Hash     string `json:"hash,omitempty"`
	Type     string `json:"type,omitempty"`
	Target   string `json:"target,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Group    string `json:"group,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Created  int64  `json:"created,omitempty"`
	Accessed int64  `json:"accessed,omitempty"`
	Modified int64  `json:"modified,omitempty"`
	Changed  int64  `json:"changed,omitempty"`
	Mode     int64  `json:"mode,omitempty"`
	Uid      int64  `json:"uid,omitempty"`
	Gid      int64  `json:"gid,omitempty"`
	DevMajor int64  `json:"devmajor,omitempty"`
	DevMinor int64  `json:"devminor,omitempty"`
}

func New(p string) *Catalog {
	return &Catalog{
		path: p,
	}
}

func (catalog *Catalog) Open() error {
	db, err := bolt.Open(catalog.path, 0640, nil)
	catalog.db = db
	return err
}

func (catalog *Catalog) Close() error {
	return catalog.db.Close()
}

func (catalog *Catalog) NewBackup(name string, date int64) {
}

func encode(v interface{}) []byte {
	result, _ := json.Marshal(v)
	return result
}

func mkPath(root *bolt.Bucket, name string) (*bolt.Bucket, error) {
	dir, base := path.Split(name)
	if base == "" {
		return root, nil
	} else {
		parent, err := mkPath(root, path.Clean(dir))
		if err != nil {
			return nil, err
		}
		return parent.CreateBucketIfNotExists([]byte(base))
	}
}
