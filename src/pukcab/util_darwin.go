package main

import (
	"fmt"
	"os"
	"syscall"
)

import "C"

func Attributes(file string) (result []string) {
	result = make([]string, 0)

	return
}

func Attribute(file string, name string) (result []byte) {
	result = make([]byte, 0)

	return
}

func DevMajorMinor(file string) (major int64, minor int64) {
	var st syscall.Stat_t

	syscall.Stat(file, &st)

	major = int64(st.Rdev>>24) & 0xff
	minor = int64(st.Rdev & 0xffffff)
	return
}

func IsNodump(fi os.FileInfo, file string) bool {
	if fi.Mode()&(os.ModeTemporary|os.ModeSocket) != 0 {
		return true
	}
	if !(fi.Mode().IsDir() || fi.Mode().IsRegular()) {
		return false
	}
	return false
}

// Fstype returns the name of a numeric filesystem type
func Fstype(t uint64) string {
	switch t {
	case 0x11:
		return "HFS+"
	case 0x13:
		return "devfs"
	case 0x14:
		return "autofs"
	case 0x16:
		return "AFP"
	case 0x1c:
		return "FAT"
	case 0x1d:
		return "exFAT"
	default:
		return fmt.Sprintf("0x%x", t)
	}
}
