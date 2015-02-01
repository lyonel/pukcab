package main

import (
	"bytes"
	"syscall"

	"github.com/antage/mntent"
)

const MNT_WAIT = 1

func tostring(s []int8) string {
	result := make([]byte, len(s))

	for i, b := range s {
		result[i] = byte(b)
	}
	return string(bytes.Trim(result, "\x00"))
}

func loadmtab() ([]*mntent.Entry, error) {
	n, err := syscall.Getfsstat(nil, MNT_WAIT)
	if err != nil {
		return nil, err
	}

	data := make([]syscall.Statfs_t, n)
	if n, err = syscall.Getfsstat(data, MNT_WAIT); err == nil {
		result := make([]*mntent.Entry, n)

		for i, f := range data {
			result[i] = &mntent.Entry{}
			result[i].Directory = tostring(f.Mntonname[:])
			result[i].Name = tostring(f.Mntfromname[:])
			result[i].Types = []string{tostring(f.Fstypename[:])}
			result[i].Options = []string{}
		}

		return result, nil
	} else {
		return nil, err
	}
}
