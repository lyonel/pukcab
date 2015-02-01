package main

import (
	"github.com/antage/mntent"
)

func loadmtab() ([]*mntent.Entry, error) {
	return mntent.Parse("/etc/mtab")
}
