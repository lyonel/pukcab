package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

/*
#include <linux/fs.h>

static int isnodump(int fd)
{
  int flags = 0;
  if(ioctl(fd, FS_IOC_GETFLAGS, &flags) < 0) {
    return 0;
  }

  return flags & FS_NODUMP_FL;
}
*/
import "C"

func nullTermToStrings(buf []byte) (result []string) {
	offset := 0
	for index, b := range buf {
		if b == 0 {
			result = append(result, string(buf[offset:index]))
			offset = index + 1
		}
	}
	return
}

// Strip off "user." prefixes from attribute names.
func stripUserPrefix(s []string) []string {
	for i, a := range s {
		if strings.HasPrefix(a, "user.") {
			s[i] = a[5:]
		}
	}
	return s
}

func Attributes(file string) (result []string) {
	if size, err := syscall.Listxattr(file, nil); err == nil {
		buf := make([]byte, size)

		if _, err = syscall.Listxattr(file, buf); err == nil {
			result = stripUserPrefix(nullTermToStrings(buf))
		}
	}

	return
}

func Attribute(file string, name string) (result []byte) {
	if !strings.Contains(name, ".") {
		name = "user." + name
	}
	if size, err := syscall.Getxattr(file, name, nil); err == nil {
		result = make([]byte, size)
		syscall.Getxattr(file, name, result)
	}

	return
}

func DevMajorMinor(file string) (major int64, minor int64) {
	var st syscall.Stat_t

	syscall.Stat(file, &st)

	major = (int64(st.Rdev>>8) & 0xfff) | (int64(st.Rdev>>32) & ^0xfff)
	minor = int64(st.Rdev&0xff) | (int64(st.Rdev>>12) & ^0xff)
	return
}

func IsNodump(fi os.FileInfo, file string) bool {
	if fi.Mode()&(os.ModeTemporary|os.ModeSocket) != 0 {
		return true
	}
	if !(fi.Mode().IsDir() || fi.Mode().IsRegular()) {
		return false
	}

	if f, err := os.Open(file); err != nil {
		return true
	} else {
		defer f.Close()
		return C.isnodump(C.int(f.Fd())) != 0
	}
}

func Fstype(t uint64) string {
	switch t & 0xffffffff {
	case 0xEF53:
		return "ext4"
	case 0x4d44:
		return "FAT"
	case 0x6969:
		return "NFS"
	case 0x517B:
		return "SMB"
	case 0x01161970:
		return "GFS2"
	case 0x65735546:
		return "FUSE"
	case 0x9123683e:
		return "Btrfs"
	case 0x52654973:
		return "ReiserFS"
	case 0x3153464a:
		return "JFS"
	case 0x58465342:
		return "XFS"
	case 0xa501FCF5:
		return "VxFS"
	case 0x01021994:
		return "tmpfs"
	default:
		return fmt.Sprintf("0x%x", t)
	}
}
