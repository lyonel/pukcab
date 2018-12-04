package main

var defaultInclude = []string{"ext2", "ext3", "ext4", "btrfs", "xfs", "jfs", "vfat", "zfs"}
var defaultExclude = []string{"/proc", "/sys", "/selinux", "tmpfs", "./.nobackup", "/var/cache/yum", "/var/cache/dnf"}
