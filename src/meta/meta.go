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

type Tx struct {
	catalog *Catalog
	tx      *bolt.Tx
	backups *bolt.Bucket
}

type Backup struct {
	Name         string `json:"name,omitempty"`
	Schedule     string `json:"schedule,omitempty"`
	Date         int64  `json:"date,omitempty"`
	Finished     int64  `json:"finished,omitempty"`
	Files        int64  `json:"files,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Lastmodified int64  `json:"lastmodified,omitempty"`

	tx     *Tx
	bucket *bolt.Bucket
}

type File struct {
	Path     string `json:"-"`
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

var (
	ErrNotOpen  = bolt.ErrDatabaseNotOpen
	ErrOpen     = bolt.ErrDatabaseOpen
	ErrNotFound = bolt.ErrBucketNotFound
	ErrExists   = bolt.ErrBucketExists
)

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

func (catalog *Catalog) Begin(writable bool) (*Tx, error) {
	tx, err := catalog.db.Begin(writable)
	result := &Tx{
		catalog: catalog,
		tx:      tx,
	}
	if writable {
		result.backups, err = result.tx.CreateBucketIfNotExists([]byte("backups"))
	} else {
		if result.backups = result.tx.Bucket([]byte("backups")); result.backups == nil {
			err = ErrNotFound
		}
	}
	return result, err
}

func (catalog *Catalog) transaction(writable bool, fn func(*Tx) error) error {
	if tx, err := catalog.Begin(writable); err == nil {
		if err = fn(tx); err == nil {
			return tx.Commit()
		} else {
			return tx.Rollback()
		}
	} else {
		return err
	}
}

func (catalog *Catalog) Update(fn func(*Tx) error) error {
	return catalog.transaction(true, fn)
}

func (catalog *Catalog) View(fn func(*Tx) error) error {
	return catalog.transaction(false, fn)
}

func (transaction *Tx) Commit() error {
	return transaction.tx.Commit()
}

func (transaction *Tx) Rollback() error {
	return transaction.tx.Rollback()
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

func store(root *bolt.Bucket, name string, mf *File) error {
	cwd, err := mkPath(root, name)
	if err != nil {
		return err
	}

	if err := cwd.Put([]byte("."), encode(mf)); err != nil {
		return err
	}
	return nil
}

func (transaction *Tx) Backup(date int64) (*Backup, error) {
	if transaction.backups == nil {
		return nil, ErrNotOpen
	}
	if bucket := transaction.backups.Bucket(encode(date)); bucket == nil {
		return nil, ErrNotFound
	} else {
		backup := &Backup{
			Date:   date,
			tx:     transaction,
			bucket: bucket,
		}

		err := json.Unmarshal(bucket.Get([]byte("info")), backup)
		return backup, err
	}
}

func (transaction *Tx) CreateBackup(date int64) (*Backup, error) {
	if transaction.backups == nil {
		return nil, ErrNotOpen
	}
	if bucket, err := transaction.backups.CreateBucket(encode(date)); err != nil {
		return nil, err
	} else {
		backup := &Backup{
			Date:   date,
			tx:     transaction,
			bucket: bucket,
		}

		err = bucket.Put([]byte("info"), encode(backup))
		return backup, err
	}
}

func (transaction *Tx) CreateBackupIfNotExists(date int64) (*Backup, error) {
	if transaction.backups == nil {
		return nil, ErrNotOpen
	}
	if bucket, err := transaction.backups.CreateBucketIfNotExists(encode(date)); err != nil {
		return nil, err
	} else {
		backup := &Backup{
			Date:   date,
			tx:     transaction,
			bucket: bucket,
		}

		err = bucket.Put([]byte("info"), encode(backup))
		return backup, err
	}
}

func (transaction *Tx) Delete(date int64) error {
	if transaction.backups == nil {
		return ErrNotOpen
	}
	return transaction.backups.DeleteBucket(encode(date))
}

func (transaction *Tx) ForEach(fn func(*Backup) error) error {
	return nil
}

func (backup *Backup) Save() error {
	if backup.bucket == nil {
		return ErrNotOpen
	}
	return backup.bucket.Put([]byte("info"), encode(backup))
}

func (backup *Backup) Delete() error {
	if backup.tx == nil {
		return ErrNotOpen
	} else {
		return backup.tx.Delete(backup.Date)
	}
}

func (backup *Backup) File(path string) (*File, error) {
	return nil, nil
}

func (backup *Backup) AddFile(file *File) error {
	if backup.bucket == nil {
		return ErrNotOpen
	}
	return store(backup.bucket, file.Path, file)
}

func (backup *Backup) ForEach(fn func(*File) error) error {
	return nil
}
