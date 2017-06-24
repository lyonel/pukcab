// +build linux darwin freebsd openbsd
// +build !windows
// +build cgo

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

/*
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <grp.h>
#include <stdlib.h>

static char *getusername(uid_t uid)
{
  struct passwd *pw = getpwuid(uid);

  if(!pw)
	return "";
  else
	return pw->pw_name;
}

static char *getgroupname(gid_t gid)
{
  struct group *gr = getgrgid(gid);

  if(!gr)
	return "";
  else
	return gr->gr_name;
}

static int setugid(uid_t uid, gid_t gid)
{
  setgid(gid);
  return setuid(uid);
}

static void daemonize()
{
  setpgid(0, 0);
}

*/
import "C"

// IsATTY checks whether a file is a TTY
func IsATTY(f *os.File) bool {
	return C.isatty(C.int(f.Fd())) != 0
}

// Username returns the username that corresponds to a numeric user ID
func Username(uid int) (username string) {
	return C.GoString(C.getusername(C.uid_t(uid)))
}

// Groupname returns the groupname that corresponds to a numeric user ID
func Groupname(gid int) (groupname string) {
	return C.GoString(C.getgroupname(C.gid_t(gid)))
}

// Passwd represents a passwd entry
type Passwd struct {
	Name  string
	Uid   int
	Gid   int
	Dir   string
	Shell string
}

// Getpwnam returns the passwd entry that corresponds to a username
func Getpwnam(name string) (pw *Passwd, err error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	if cpw := C.getpwnam(cname); cpw != nil {
		pw = &Passwd{
			Name:  C.GoString(cpw.pw_name),
			Uid:   int(cpw.pw_uid),
			Gid:   int(cpw.pw_gid),
			Dir:   C.GoString(cpw.pw_dir),
			Shell: C.GoString(cpw.pw_shell)}
	} else {
		err = fmt.Errorf("Unknown user %s", name)
	}

	return
}

// Uid returns the numeric user ID that corresponds to a username (or -1 if none does)
func Uid(username string) int {
	if pw, err := Getpwnam(name); err == nil {
		return pw.Uid
	}
	return -1
}

// Gid returns the numeric group ID that corresponds to a groupname (or -1 if none does)
func Gid(username string) int {
	if pw, err := Getpwnam(name); err == nil {
		return pw.Gid
	}
	return -1
}

// Impersonate switch the running process to another user (requires root privileges)
func Impersonate(name string) error {
	if pw, err := Getpwnam(name); err == nil {
		os.Setenv("USER", pw.Name)
		os.Setenv("LOGNAME", name)
		os.Chdir(pw.Dir)

		os.Setenv("HOME", pw.Dir)
		C.setugid(C.uid_t(pw.Uid), C.gid_t(pw.Gid))
	} else {
		return err
	}
	return nil
}

// ExitCode returns the exit code of a child process
func ExitCode(s *os.ProcessState) int {
	if r, ok := s.Sys().(syscall.WaitStatus); ok {
		return r.ExitStatus()
	}
	return -1
}

// LoadAvg returns the system load average
func LoadAvg() (result float64) {
	C.getloadavg((*C.double)(unsafe.Pointer(&result)), 1)

	return
}

// Daemonize turns the current process into a daemon (i.e. background process)
func Daemonize() {
	C.daemonize()
}

// Renice reduces the current process' priority
func Renice() {
	syscall.Setpriority(syscall.PRIO_PGRP, 0, 5)
}
