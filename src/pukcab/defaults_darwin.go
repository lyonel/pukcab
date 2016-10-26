package main

var defaultInclude = []string{"hfs"}
var defaultExclude = []string{
	"afpfs",
	"autofs",
	"devfs",
	"smbfs",
	"/.fseventsd",
	"/Volumes/*/.fseventsd",
	"/.Spotlight-V100",
	"/Volumes/*/.Spotlight-V100",
	"/.Trashes",
	"/Volumes/*.Trashes",
	"/.MobileBackups",
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
	"/Volumes/*/Backups.backupdb",
	"./.nobackup",
}
