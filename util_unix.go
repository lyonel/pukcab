package main

import (
	"os"
)

/*
#include <unistd.h>
*/
import "C"

func IsATTY(f *os.File) bool {
	return bool(C.isatty(C.int(f.Fd())) != 0)
}
