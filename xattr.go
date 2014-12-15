package main

import (
	"strings"
	"syscall"
)

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
