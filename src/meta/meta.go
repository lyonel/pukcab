package meta

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"path"
)

// Index represents a collection of backups
//
// All the functions on Index will return a ErrNotOpen if accessed before Open() is called.
type Index struct {
	db   *bolt.DB
	path string
}

// Tx represents a read-only or read/write transaction on an index.
//
// Read-only transactions can be used for retrieving information about backups and individual files.
// Read/write transactions can create and remove backups or individual files.
type Tx struct {
	index   *Index
	tx      *bolt.Tx
	backups *bolt.Bucket
}

// Backup represents a backup set.
//
// Backup sets are uniquely identified by their date (a 64 bit integer UNIX epoch timestamp) and contain files stored in a tree-like hierarchy.
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

// File represents an individual file.
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

// New creates a new index associated to a given file.
// The file is not accessed nor created until Open() is called.
func New(p string) *Index {
	return &Index{
		path: p,
	}
}

// Open opens an index.
// If the file does not exist then it will be created automatically.
func (index *Index) Open() error {
	db, err := bolt.Open(index.path, 0640, nil)
	index.db = db
	return err
}

// Close releases all index resources.
// All transactions must be closed before closing the index.
func (index *Index) Close() error {
	return index.db.Close()
}

// Begin starts a new transaction.
// Multiple read-only transactions can be used concurrently but only one write transaction can be used at a time.
// Starting multiple write transactions will cause the calls to block and be serialized until the current write transaction finishes.
func (index *Index) Begin(writable bool) (*Tx, error) {
	tx, err := index.db.Begin(writable)
	result := &Tx{
		index: index,
		tx:    tx,
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

func (index *Index) transaction(writable bool, fn func(*Tx) error) error {
	if tx, err := index.Begin(writable); err == nil {
		if err = fn(tx); err == nil {
			if writable {
				return tx.Commit()
			}
			return tx.Rollback()
		}
		tx.Rollback()
		return err
	} else {
		return err
	}
}

// Update executes a function within the context of a read-write managed transaction.
// If no error is returned from the function then the transaction is committed.
// If an error is returned then the entire transaction is rolled back.
// Any error that is returned from the function or returned from the commit is returned from the Update() method.
//
// Attempting to manually commit or rollback within the function will cause a panic.
func (index *Index) Update(fn func(*Tx) error) error {
	return index.transaction(true, fn)
}

// View executes a function within the context of a managed read-only transaction.
// Any error that is returned from the function is returned from the View() method.
//
// Attempting to manually rollback within the function will cause a panic.
func (index *Index) View(fn func(*Tx) error) error {
	return index.transaction(false, fn)
}

// Commit writes all changes to disk.
// Returns an error if a disk write error occurs, or if Commit is called on a read-only transaction.
func (transaction *Tx) Commit() error {
	return transaction.tx.Commit()
}

// Rollback closes the transaction and ignores all previous updates.
// Read-only transactions must be rolled back and not committed.
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

func cd(root *bolt.Bucket, name string) *bolt.Bucket {
	dir, base := path.Split(name)
	if base == "" {
		return root
	} else {
		parent := cd(root, path.Clean(dir))
		if parent == nil {
			return nil
		}
		return parent.Bucket([]byte(base))
	}
}

func store(bucket *bolt.Bucket, name string, mf *File) error {
	root, err := bucket.CreateBucketIfNotExists([]byte("files"))
	if err != nil {
		return err
	}
	cwd, err := mkPath(root, name)
	if err != nil {
		return err
	}

	if err := cwd.Put([]byte("."), encode(mf)); err != nil {
		return err
	}
	return nil
}

// Backup retrieves an existing backup identified by its date.
// Returns an error if the backup does not exist.
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

// CreateBackup creates a new backup identified by its date.
// Returns an error if the backup already exists.
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

// CreateBackupIfNotExists creates a new backup or retrieves an existing backup.
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

// Delete deletes an existing backup identified by its date.
// Returns an error if the backup does not exist.
func (transaction *Tx) Delete(date int64) error {
	if transaction.backups == nil {
		return ErrNotOpen
	}
	return transaction.backups.DeleteBucket(encode(date))
}

// ForEach executes a function for each backup.
// If the provided function returns an error then the iteration is stopped and the error is returned to the caller.
// The provided function must not modify the backup; this will result in undefined behavior.
func (transaction *Tx) ForEach(fn func(*Backup) error) error {
	if transaction.backups == nil {
		return ErrNotOpen
	}

	cursor := transaction.backups.Cursor()
	for id, _ := cursor.First(); id != nil; id, _ = cursor.Next() {
		var date int64
		if err := json.Unmarshal(id, &date); err == nil {
			if backup, err := transaction.Backup(date); err == nil {
				err = fn(backup)
			}
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// Save records any modification of a backup to the index.
func (backup *Backup) Save() error {
	if backup.bucket == nil {
		return ErrNotOpen
	}
	return backup.bucket.Put([]byte("info"), encode(backup))
}

// Delete deletes a backup.
func (backup *Backup) Delete() error {
	if backup.tx == nil {
		return ErrNotOpen
	} else {
		return backup.tx.Delete(backup.Date)
	}
}

// File retrieves an existing file within a backup.
// Returns an error if the file does not exist.
func (backup *Backup) File(filepath string) (*File, error) {
	if backup.bucket == nil {
		return nil, ErrNotOpen
	}
	root := backup.bucket.Bucket([]byte("files"))
	if root == nil {
		return nil, ErrNotFound
	}
	if bucket := cd(root, filepath); bucket != nil {
		file := &File{
			Path: path.Clean(filepath),
		}

		err := json.Unmarshal(bucket.Get([]byte(".")), file)
		return file, err
	}
	return nil, ErrNotFound
}

// AddFile records or updates a file within a backup.
func (backup *Backup) AddFile(file *File) error {
	if backup.bucket == nil {
		return ErrNotOpen
	}
	return store(backup.bucket, file.Path, file)
}

func ls(prefix string, bucket *bolt.Bucket, fn func(*File) error) error {
	if bucket == nil {
		return ErrNotFound
	}

	file := &File{
		Path: path.Clean(prefix),
	}
	fileinfo := bucket.Get([]byte("."))
	if len(fileinfo) > 0 {
		if err := json.Unmarshal(fileinfo, file); err == nil {
			err = fn(file)
			if err != nil {
				return err
			}
		}
	}

	cursor := bucket.Cursor()
	if cursor == nil {
		return ErrNotFound
	}
	for name, empty := cursor.First(); name != nil; name, empty = cursor.Next() {
		if empty == nil { // subdirectory
			if err := ls(path.Join(prefix, string(name)), bucket.Bucket(name), fn); err != nil {
				return err
			}
		}
	}

	return nil
}

// ForEach executes a function for each file within a backup.
// If the provided function returns an error then the iteration is stopped and the error is returned to the caller.
// The provided function must not modify the backup; this will result in undefined behavior.
func (backup *Backup) ForEach(fn func(*File) error) error {
	if backup.bucket == nil {
		return ErrNotOpen
	}
	root := backup.bucket.Bucket([]byte("files"))
	if root == nil {
		return ErrNotFound
	}
	return ls("/", root, fn)
}
