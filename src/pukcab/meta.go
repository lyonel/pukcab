package main

import (
	"encoding/json"
	"os"
	"path"
	"sort"
	"strconv"
	"time"

	"pukcab/tar"

	"ezix.org/src/pkg/git"
)

// BackupMeta represents catalog metadata for a backup set
type BackupMeta struct {
	Date         BackupID `json:"-"`
	Name         string   `json:"-"`
	Schedule     string   `json:"schedule,omitempty"`
	Files        int64    `json:"files,omitempty"`
	Size         int64    `json:"size,omitempty"`
	Finished     int64    `json:"finished,omitempty"`
	LastModified int64    `json:"-"`
}

// Meta represents catalog metadata for a backup entry (file or directory)
type Meta struct {
	Path       string            `json:"-"`
	Hash       string            `json:"hash,omitempty"`
	Type       string            `json:"type,omitempty"`
	Target     string            `json:"target,omitempty"`
	Owner      string            `json:"owner,omitempty"`
	Group      string            `json:"group,omitempty"`
	Size       int64             `json:"size,omitempty"`
	Created    int64             `json:"created,omitempty"`
	Accessed   int64             `json:"accessed,omitempty"`
	Modified   int64             `json:"modified,omitempty"`
	Changed    int64             `json:"changed,omitempty"`
	Mode       int64             `json:"mode,omitempty"`
	Uid        int               `json:"uid,omitempty"`
	Gid        int               `json:"gid,omitempty"`
	Devmajor   int64             `json:"devmajor,omitempty"`
	Devminor   int64             `json:"devminor,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Type returns a tar-like type byte
func Type(mode os.FileMode) byte {
	switch {
	case mode.IsDir():
		return tar.TypeDir // directory
	case mode&os.ModeSymlink != 0:
		return tar.TypeSymlink
	case mode&os.ModeDevice != 0:
		return tar.TypeBlock
	case mode&os.ModeCharDevice != 0:
		return tar.TypeChar
	case mode&os.ModeSocket != 0:
		return tar.TypeFifo
	case mode&os.ModeNamedPipe != 0:
		return tar.TypeFifo
	default:
		return tar.TypeReg // regular file
	}
}

// FileInfoMeta translates os.FileInfo into catalog metadata
func FileInfoMeta(fi os.FileInfo) Meta {
	return Meta{
		Path:     fi.Name(),
		Size:     fi.Size(),
		Type:     string(Type(fi.Mode())),
		Mode:     int64(fi.Mode()),
		Modified: fi.ModTime().Unix(),
	}
}

// Perm returns permissions of a backup entry
func (meta Meta) Perm() os.FileMode {
	return os.FileMode(meta.Mode).Perm()
}

// TarHeader translates catalog metadata into tar header
func (meta Meta) TarHeader() *tar.Header {
	switch meta.Type {
	case string(tar.TypeRegA):
		meta.Type = string(tar.TypeReg)
	case "":
		meta.Type = "?"
	}
	hdr := tar.Header{
		Name:       meta.Path,
		Mode:       meta.Mode,
		Uid:        meta.Uid,
		Gid:        meta.Gid,
		Size:       meta.Size,
		ModTime:    time.Unix(meta.Modified, 0),
		AccessTime: time.Unix(meta.Accessed, 0),
		ChangeTime: time.Unix(meta.Changed, 0),
		Typeflag:   meta.Type[0],
		Linkname:   meta.Target,
		Uname:      meta.Owner,
		Gname:      meta.Group,
		Devmajor:   meta.Devmajor,
		Devminor:   meta.Devminor,
	}
	if meta.Attributes != nil {
		hdr.Xattrs = make(map[string]string)
		for k, v := range meta.Attributes {
			hdr.Xattrs[k] = v
		}
	}
	if meta.Target == "" {
		hdr.Linkname = "."
	}
	return &hdr
}

// HeaderMeta translates tar header into catalog metadata
func HeaderMeta(h *tar.Header) Meta {
	meta := Meta{
		Path:     h.Name,
		Mode:     h.Mode,
		Uid:      h.Uid,
		Gid:      h.Gid,
		Size:     h.Size,
		Modified: h.ModTime.Unix(),
		Accessed: h.AccessTime.Unix(),
		Changed:  h.ChangeTime.Unix(),
		Type:     string(h.Typeflag),
		Target:   h.Linkname,
		Owner:    h.Uname,
		Group:    h.Gname,
		Devmajor: h.Devmajor,
		Devminor: h.Devminor,
	}
	switch meta.Type {
	case string(tar.TypeRegA):
		meta.Type = string(tar.TypeReg)
	case "":
		meta.Type = "?"
	}

	if h.Linkname == "." {
		meta.Target = ""
	}

	if meta.Type == string(tar.TypeReg) {
		meta.Hash, meta.Target = meta.Target, ""
	}

	if h.Xattrs != nil {
		meta.Attributes = make(map[string]string)
		for k, v := range h.Xattrs {
			switch k {
			case "backup.size":
				meta.Size, _ = strconv.ParseInt(v, 10, 64)
			case "backup.hash":
				meta.Hash = v
			default:
				meta.Attributes[k] = v
			}
		}
	}
	return meta
}

// JSON returns a JSON-encoded string representation of an object (or empty string if conversion failed)
func JSON(v interface{}) string {
	if b, err := json.Marshal(v); err == nil {
		return string(b) + "\n"
	} else {
		return ""
	}
}

func unixtime(t int64) time.Time {
	if t == 0 {
		return time.Time{}
	} else {
		return time.Unix(t, 0)
	}
}

// First returns the earliest backup set of a list
func First(list []Backup) (first Backup) {
	for _, b := range list {
		if first.Date == 0 || b.Date < first.Date {
			first = b
		}
	}
	return first
}

// Last returns the latest backup set of a list
func Last(list []Backup) (last Backup) {
	for _, b := range list {
		if b.Date > last.Date {
			last = b
		}
	}
	return last
}

// Finished returns all complete backup sets of a list
func Finished(list []Backup) (backups []Backup) {
	for _, b := range list {
		if !b.Finished.IsZero() {
			backups = append(backups, b)
		}
	}
	return backups
}

// Get returns a given backup set from a list
func Get(date BackupID, list []Backup) Backup {
	for _, b := range list {
		if b.Date == date {
			return b
		}
	}
	return Backup{}
}

// Before returns all backups sets older than a given date from a list
func Before(date BackupID, list []Backup) (backups []Backup) {
	for _, b := range list {
		if b.Date <= date {
			backups = append(backups, b)
		}
	}
	return backups
}

// Filter only returns backup sets for a given name/schedule from a list
func Filter(name string, schedule string, list []Backup) (backups []Backup) {
	// empty filter = no filter
	if name == "" {
		name = "*"
	}
	if schedule == "" {
		schedule = "*"
	}

	for _, b := range list {
		nameok, _ := path.Match(name, b.Name)
		scheduleok, _ := path.Match(schedule, b.Schedule)
		if nameok && (b.Schedule == "" || scheduleok) {
			backups = append(backups, b)
		}
	}

	return backups
}

// After returns all backup sets more recent than a given date from a list
func After(date BackupID, list []Backup) (backups []Backup) {
	for _, b := range list {
		if b.Date >= date {
			backups = append(backups, b)
		}
	}
	return backups
}

// Backups returns a list of backup sets for a given name/schedule from a catalog
func Backups(repository *git.Repository, name string, schedule string) (list []Backup) {
	// empty filter = no filter
	if name == "" {
		name = "*"
	}
	if schedule == "" {
		schedule = "*"
	}

	backups := make(map[BackupID]BackupMeta)
	for _, ref := range repository.Tags() {
		if date, err := strconv.ParseInt(path.Base(ref.Name()), 10, 64); err == nil && date > 0 {
			b := BackupMeta{
				Date: BackupID(date),
			}
			if obj, err := repository.Object(ref); err == nil {
				switch tag := obj.(type) {
				case git.Commit:
					b.LastModified = tag.Author().Date().Unix()
				case git.Tag:
					json.Unmarshal([]byte(tag.Text()), &b)
				}
			}
			backups[b.Date] = b
		}
	}

	branches := repository.Branches()
	for d := range backups {
		ref := repository.Reference(d.String())
		for _, branch := range branches {
			if repository.Ancestor(ref, branch) {
				meta := backups[d]
				meta.Name = path.Base(branch.Name())
				backups[d] = meta
			}
		}
	}

	for _, b := range backups {
		nameok, _ := path.Match(name, b.Name)
		scheduleok, _ := path.Match(schedule, b.Schedule)
		if nameok && (b.Schedule == "" || scheduleok) {
			list = append(list, Backup{
				Date:         b.Date,
				Name:         b.Name,
				Schedule:     b.Schedule,
				Size:         b.Size,
				Files:        b.Files,
				Started:      unixtime(int64(b.Date)),
				Finished:     unixtime(b.Finished),
				LastModified: unixtime(b.LastModified),
			})
		}
	}

	sort.Sort(bydate(list))
	return list
}

type bydate []Backup

func (a bydate) Len() int {
	return len(a)
}

func (a bydate) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a bydate) Less(i, j int) bool {
	return a[i].Date < a[j].Date
}
