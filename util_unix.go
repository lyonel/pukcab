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

func Impersonate(name string) error {
	if pw, err := Getpwnam(name); err != nil {
		return err
	} else {
		syscall.Setgid(pw.Gid)
		if err = syscall.Setuid(pw.Uid); err == nil {
			os.Setenv("USER", pw.Name)
			os.Setenv("LOGNAME", name)
			if os.Chdir(pw.Dir) == nil {
				os.Setenv("HOME", pw.Dir)
			}
		}
		return err
	}
}
