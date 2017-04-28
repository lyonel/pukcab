package main

import (
	"encoding/json"
	"os"
	"path"
	"sort"
	"strconv"
	"time"

	"ezix.org/src/pkg/git"
	"pukcab/tar"
)

type BackupMeta struct {
	Date         BackupID `json:"-"`
	Name         string   `json:"-"`
	Schedule     string   `json:"schedule,omitempty"`
	Files        int64    `json:"files,omitempty"`
	Size         int64    `json:"size,omitempty"`
	Finished     int64    `json:"finished,omitempty"`
	LastModified int64    `json:"-"`
}

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

func Type(mode os.FileMode) rune {
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

func FileInfoMeta(fi os.FileInfo) Meta {
	return Meta{
		Path:     fi.Name(),
		Size:     fi.Size(),
		Type:     string(Type(fi.Mode())),
		Mode:     int64(fi.Mode()),
		Modified: fi.ModTime().Unix(),
	}
}

func (meta Meta) Perm() os.FileMode {
	return os.FileMode(meta.Mode).Perm()
}

func HeaderMeta(h *tar.Header) Meta {
	return Meta{
		Path:       h.Name,
		Mode:       h.Mode,
		Uid:        h.Uid,
		Gid:        h.Gid,
		Size:       h.Size,
		Modified:   h.ModTime.Unix(),
		Accessed:   h.AccessTime.Unix(),
		Changed:    h.ChangeTime.Unix(),
		Type:       string(rune(h.Typeflag)),
		Target:     h.Linkname,
		Owner:      h.Uname,
		Group:      h.Gname,
		Devmajor:   h.Devmajor,
		Devminor:   h.Devminor,
		Attributes: h.Xattrs,
	}
}

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

func First(list []Backup) (first Backup) {
	for _, b := range list {
		if first.Date == 0 || b.Date < first.Date {
			first = b
		}
	}
	return first
}

func Last(list []Backup) (last Backup) {
	for _, b := range list {
		if b.Date > last.Date {
			last = b
		}
	}
	return last
}

func Finished(list []Backup) (backups []Backup) {
	for _, b := range list {
		if !b.Finished.IsZero() {
			backups = append(backups, b)
		}
	}
	return backups
}

func Get(date BackupID, list []Backup) Backup {
	for _, b := range list {
		if b.Date == date {
			return b
		}
	}
	return Backup{}
}

func Before(date BackupID, list []Backup) (backups []Backup) {
	for _, b := range list {
		if b.Date <= date {
			backups = append(backups, b)
		}
	}
	return backups
}

func After(date BackupID, list []Backup) (backups []Backup) {
	for _, b := range list {
		if b.Date >= date {
			backups = append(backups, b)
		}
	}
	return backups
}

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
