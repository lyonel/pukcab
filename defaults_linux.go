package main

const gzipOS = 3 // UNIX, cf. http://www.gzip.org/zlib/rfc-gzip.html

var defaultInclude = []string{"ext2", "ext3", "ext4", "btrfs", "xfs", "jfs", "vfat"}
var defaultExclude = []string{"/proc", "/sys", "/selinux", "tmpfs"}
