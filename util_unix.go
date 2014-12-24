package main

import (
	"os"
)

/*
#include <unistd.h>
*/
import "C"

func IsATTY(f *os.File) bool {
	return C.isatty(C.int(f.Fd())) != 0
}
