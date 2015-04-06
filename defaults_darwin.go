package main

var defaultInclude = []string{"hfs"}
var defaultExclude = []string{
	"afpfs",
	"autofs",
	"devfs",
	"smbfs",
	"/.fseventsd",
	"/.Spotlight-V100",
	"/.Trashes",
	"/.vol",
	"/private/tmp",
	"/private/var/db/BootCaches",
	"/private/var/db/systemstats",
	"/private/var/folders",
	"/private/var/tmp",
	"/private/var/vm",
	"/Library/Caches",
	"/System/Library/Caches",
	"/Users/*/Library/Caches",
	"./fseventsd-uuid",
	"./.nobackup",
}
