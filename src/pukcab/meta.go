package main

import (
	"encoding/json"
	"os"

	"pukcab/tar"
)

type BackupMeta struct {
	Date     BackupID `json:"-"`
	Name     string   `json:"-"`
	Schedule string   `json:"schedule,omitempty"`
	Files    int64    `json:"files,omitempty"`
	Size     int64    `json:"size,omitempty"`
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
