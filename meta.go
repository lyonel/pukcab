package main

import (
	"github.com/boltdb/bolt"
)

type Meta struct {
	db *bolt.DB
}

type BackupMetadata struct {
	Name         string `json:"name,omitempty"`
	Schedule     string `json:"schedule,omitempty"`
	Date         int64  `json:"date,omitempty"`
	Finished     int64  `json:"finished,omitempty"`
	Files        int64  `json:"files,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Lastmodified int64  `json:"lastmodified,omitempty"`
}

type FileMetadata struct {
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
