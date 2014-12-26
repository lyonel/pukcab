package main

import (
	"os"
)

/*
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <grp.h>

static char *getusername(int uid)
{
  struct passwd *pw = getpwuid(uid);

  if(!pw)
	return "";
  else
	return pw->pw_name;
}

static char *getgroupname(int gid)
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
	return C.GoString(C.getusername(C.int(uid)))
}

func Groupname(gid int) (groupname string) {
	return C.GoString(C.getgroupname(C.int(gid)))
}
