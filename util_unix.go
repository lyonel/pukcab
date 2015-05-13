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

static daemonize()
{
  setpgid(0, 0);
}

*/
import "C"

func IsATTY(f *os.File) bool {
	return C.isatty(C.int(f.Fd())) != 0
}

func Username(uid int) (username string) {
	return C.GoString(C.getusername(C.uid_t(uid)))
}

func Groupname(gid int) (groupname string) {
	return C.GoString(C.getgroupname(C.gid_t(gid)))
}

type Passwd struct {
	Name  string
	Uid   int
	Gid   int
	Dir   string
	Shell string
}

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

func Uid(username string) int {
	if pw, err := Getpwnam(name); err != nil {
		return -1
	} else {
		return pw.Uid
	}
}

func Gid(username string) int {
	if pw, err := Getpwnam(name); err != nil {
		return -1
	} else {
		return pw.Gid
	}
}

func Impersonate(name string) error {
	if pw, err := Getpwnam(name); err != nil {
		return err
	} else {
		os.Setenv("USER", pw.Name)
		os.Setenv("LOGNAME", name)
		os.Chdir(pw.Dir)

		os.Setenv("HOME", pw.Dir)
		C.setugid(C.uid_t(pw.Uid), C.gid_t(pw.Gid))
		return err
	}
}

func ExitCode(s *os.ProcessState) int {
	if r, ok := s.Sys().(syscall.WaitStatus); ok {
		return r.ExitStatus()
	} else {
		return -1
	}
}

func LoadAvg() (result float64) {

	C.getloadavg((*C.double)(unsafe.Pointer(&result)), 1)

	return
}

func Daemonize() {
	C.daemonize()
}
