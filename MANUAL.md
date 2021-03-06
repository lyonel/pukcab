% Pukcab User's Guide
%
% March 2015

Introduction
============

`pukcab` is a lightweight, single-binary backup system for UNIX / Linux systems that stores de-duplicated, compressed and incremental backups on a remote server using just an SSH connection.

De-duplication happens not only between incremental backups of the same system but also between different systems. For example, it allows you to perform full backups of systems running the same OS with only minimal disk space for each additional system^[Basically, only configuration files and user data require disk space --- that's a blatant lie, the catalog uses disk space, too (but hopefully much less).].

Intended use
------------

`pukcab` doesn't compare to professional-grade backup systems, *don't expect it to be able to backup thousands of systems or dozens of terabytes of data*.

It is, however, perfectly suitable for home users, hobbyists, UNIX / Linux enthusiasts or small tech-savy shops who want a flexible, yet dead-easy to setup and operate backup system with no bigger expenses than just a Linux box with a few terabytes of storage.

Little to no configuration should ever be required to satisfy most needs:

 * just run `pukcab backup` every night on your systems
 * full-system backups should be preferred, thanks to sensible OS-dependent default exclusions
 * automatic daily/weekly/monthly/yearly retention policies should keep enough backups around

Features
--------

 * lightweight (just 1 binary to be installed on both the client and the server)
 * easy to install (only 1 username with SSH connectivity is required to set up a server)
 * flexible configuration
 * sensible defaults
 * automatic retention schedules
 * incremental/full backups
 * data de-duplication
 * data compression
 * (optional) web interface

Requirements
============

`pukcab` was written with UNIX-like operating systems in mind, it is therefore currently unsupported on Microsoft Windows^[This might change in the future, though.].

`pukcab` has been tested on the following operating systems and platforms^[The main development platform is Fedora Linux x86-64.]:

 * Linux 4.*x* on 64-bit Intel or AMD processors
 * Linux 4.*x* on 32-bit Intel or AMD processors^[Some old Pentium III machines may misbehave.]
 * Linux 3.*x* on 32-bit ARM processors
 * Mac OS X on 64-bit Intel processors

Backup server
-------------

To run a `pukcab` backup server, you will need:

 * SSH server
 * dedicated user (recommended)
 * disk space
 * scalable filesystem
 * enough memory or swap space (running without swap space is *not* recommended)

The "scalable filesystem" requirement might seem surprising but, to store backups, you will need a modern filesystem that can gracefully handle thousands and thousands of files per directory, sometimes big, sometimes small.

On Linux, [XFS], [ext4] and [Btrfs] are known to work. FAT, NTFS or HFS cannot and must not be used.

Clients
-------

The requirements for a client are very limited. In short, nearly any Linux/OS X box will do.

 * SSH client
 * functional `tar` command (tested with [GNU tar](http://www.gnu.org/software/tar/), should work with [BSD (libarchive) tar](http://www.libarchive.org/) and [Jörg Schilling's star](http://sourceforge.net/projects/s-tar/)). Caution: the `tar` command of [BusyBox] is known to have issues that will prevent it from restoring files.
 * `root` access (if you want to backup files other than yours)

Installation
============

Just copy the `pukcab` binary[^RPM] into your path (`/usr/bin/pukcab` will be just fine) on the backup server and each client.

OS              Platform       Width      Packages
--------     -------------   ---------    ------------------------------------------------------------------------
Linux        x86-64            64-bit       [ZIP](http://ezix.org/download/?package=x86-64.linux.pukcab.ezix.org)
Linux        i686              32-bit       [ZIP](http://ezix.org/download/?package=i686.linux.pukcab.ezix.org)
Linux        ARM               32-bit       [ZIP](http://ezix.org/download/?package=arm.linux.pukcab.ezix.org)
Mac OS X     x86-64            64-bit       [ZIP](http://ezix.org/download/?package=osx.pukcab.ezix.org)
*any*        *any*             *any*        [Source](http://ezix.org/download/?package=pukcab.ezix.org)[^Go]
--------     -------------   ---------    ------------------------------------------------------------------------

For RPM-based Linux distributions, YUM repositories are available:

 * [stable](http://ezix.org/software/pukcab.repo)
 * [beta](http://ezix.org/software/pukcab-beta.repo)

Copy the `.repo` file into `/etc/yum.repos.d` and do either:

 * `sudo yum install pukcab-server`
 * `sudo yum install pukcab-client`

[^RPM]: Linux users should prefer [RPM packages](http://ezix.org/download/?package=rpm.pukcab.ezix.org) or check if their distribution already includes `pukcab`.
[^Go]: To rebuild `pukcab`, you will need a [Go](http://golang.org) development environment (and some courage).

On the backup server
--------------------

1. create a dedicated user, if necessary (usually called `pukcab`) -- this user does not need specific privileges (i.e. do *NOT* use `root`)
2. allow key-based SSH login for that user
3. *optional*: allow password-based SSH login and set a password for the dedicated user (if you want to be able to register new clients using that password)

On the clients (manual)
----------------------------------------

1. [create SSH keys] for the user which will launch the backup (most probably `root`)
2. add the user's public key to the dedicated user's `authorized_keys` on the backup server

On the clients (password registration)
----------------------------------------

1. [create SSH keys] for the user which will launch the backup (most probably `root`)
2. [register] to the backup server

Configuration
=============

`pukcab` is configured with a simple [INI-like text file](https://en.wikipedia.org/wiki/INI_file):

~~~~~~~~~~~~~~~~~~~
; comment
name1 = number
name2 = "text value"
name3 = [ "list", "of", "text", "values" ]
[section1]
name4 = "text value"
name5 = number
[section2]
name6 = "text value"
...
~~~~~~~~~~~~~~~~~~~

The default is to read `/etc/pukcab.conf` then `~/.pukcabrc` (which means that this user-defined file can override values set in the global configuration file).

Both client and server use the same configuration file format and location, only the values determine the client or server role (a client will have a `server` parameter set).

### Notes

 * text values must be enclosed in `"`
 * lists of values are enclosed in `[` and `]` with comma-separated items

Server
------

The `pukcab` server contains the data files for all clients (in the `vault`) and a database of all backup sets (the `catalog`).

:server configuration

parameter/section     type    default         description
-------------------- ------- ------------     -------------------------------
`user`                text    *none*          user name `pukcab` will run under (*mandatory*)
`vault`               text   `"vault"`        folder where all archive files will be created
`catalog`             text   `"catalog.db"`   name of the catalog database
`maxtries`           number   `10`            number of retries in case of concurrent client accesses
`web`                 text    *none*          auto-start the web interface on [*host*]:*port* (cf. [listen])
`webroot`             text    *none*          base URI of the web interface
`[expiration]`       section                  specify expiration of standard schedules
`daily`              number   `14`            retention (in days) of `daily` backups
`weekly`             number   `42`            retention (in days) of `weekly` backups
`monthly`            number   `365`           retention (in days) of `monthly` backups
`yearly`             number   `3650`          retention (in days) of `yearly` backups
-------------------- ------- ------------     -------------------------------

### Notes

 * `vault` and `catalog` paths can be absolute (starting with `/`) or relative to `user`'s home directory.
 * the `vault` folder must be able to store many gigabytes of data, spread over thousands of files
 * the `catalog` database may become big and must be located in a folder where `user` has write access
 * the `vault` folder **must not be used to store anything**^[`pukcab` will *silently* delete anything you may store there] else than `pukcab`'s data files; in particular, do **NOT** store the `catalog` there

### Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
; all backups will be received and stored by the 'backup' user
user="backup"
vault="/var/local/backup/vault"
catalog="/var/local/backup/catalog.db"
; keep daily backups longer (4 weeks instead of 2)
[expiration]
daily=28
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

### Scheduling expiration

This task should be run *every day*, preferably when the system is idle (or at least, not receiving backups from clients).

Use [`cron`](https://en.wikipedia.org/wiki/Cron) to schedule `pukcab expire` to run daily.

On many systems (most Linux distributions), you can also create a script in `/etc/cron.daily/pukcab-expire` with the following content:

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
#!/bin/sh

exec pukcab expire
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Client
------

The client configuration mainly focuses on what to include in the backup and what to exclude from it.

:client configuration

parameter      type      default          description
----------    ------    --------------    ----------------------------------------------------------
`user`         text     *none*            user name to use to connect (*mandatory*)
`server`       text     *none*            backup server (*mandatory*)
`port`        number    `22`              TCP port to use on the backup server
`command`      text     `"pukcab"`        command to use on the backup server
`include`      list     [OS-dependent]    what to include in the backup
`exclude`      list     [OS-dependent]    what to exclude from the backup
`tar`          text     `"tar"`           tar command to use to [restore] files
----------    ------    --------------    ----------------------------------------------------------

### `includ`ing / `exclud`ing items

When taking a backup, `pukcab` goes through several steps to determine what should be backed up

 1. get all mounted filesystems
 1. include everything that is listed in `include`
 1. exclude everything that is listed in `exclude`

To select stuff to be included or excluded, you can use the following formats:

:`include`/`exclude` filters

format           matches                                               examples
------------     ---------------------------------------------------   -----------------------------------------------
*type*           mounted filesystems of that type                      `"ext4"`, `"btrfs"`, `"procfs"`, `"tmpfs"`, `"hfs"`
`/`*path*        *path* and anything under it                          `"/usr/tmp"`, `"/var/tmp"`, `"/tmp"`
*pattern*        files matching *pattern*[^shellpattern]               `".*.swp"`, `"*.part"`, `"*.tmp"`
`./`*name*       directories containing something named *name*         `"./.nobackup"`
------------     ---------------------------------------------------   -----------------------------------------------

[^shellpattern]: Shell patterns include at least one `*` or `?` special character, with their usual meaning

### Example

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
user="backup"
server="backupserver.localdomain.net"
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

### Scheduling backups

Use [`cron`](https://en.wikipedia.org/wiki/Cron) to schedule `pukcab backup` to run whenever you want to take backups.

For daily backups, you can often just create a script in `/etc/cron.daily/pukcab` with the following content:

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
#!/bin/sh

exec pukcab backup
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

### OS-dependent defaults

`pukcab` tries to apply "sane" defaults, especially when taking a backup. In particular, it will only attempt to backup "real" filesystems and skip temporary files or pseudo-filesystems.

Under Linux, there are many exclusions caused by the extensive use of pseudo-filesystems.

:Linux defaults

parameter     default value
----------    -------------------------------------------------------------
`include`     `[ "ext2", "ext3", "ext4", "btrfs", "xfs", "jfs", "vfat" ]`
`exclude`     `[ "/proc", "/sys", "/selinux", "tmpfs", "./.nobackup" ]`
----------    -------------------------------------------------------------

For Mac OS X, the list of included filesystems is much shorter (it currently includes just the default filesystem, HFS).

:Mac OS X defaults

parameter     default value
----------    -------------------------------------------------
`include`     `[ "hfs" ]`
`exclude`     `[ "devfs", "autofs", "afpfs", "./.nobackup" ]`
----------    -------------------------------------------------

Usage
=====

Synopsis
--------

`pukcab` _command_ [ [_options_] ... ] [ [_files_] ... ]

:available commands

--------------------------- -----------------------------------------
[backup], [save]            take a new backup
[config], [cfg]             display `pukcab`'s configuration
[continue], [resume]        continue a partial backup
[delete], [purge]           delete a backup
[expire]                    apply retention schedule to old backups
[history], [versions]       list history for files
[info], [list]              list backups and files
[ping], [test]              check server connectivity
[register]                  register to backup server
[restore]                   restore files
[summary],[dashboard]       display information about backups
[vacuum]                    vault and catalog clean-up
[verify], [check]           verify files in a backup
[web]                       starts the built-in web interface
--------------------------- -----------------------------------------

`backup`
--------

The `backup` command launches a new backup:

 * creates a new backup set (and the corresponding date/id) on the [backup server](#server)
 * builds the list of files to be backed-up, based on the `include`/`exclude` configuration directives
 * sends that list to the backup server
 * computes the list of changes since the last backup (if `--full` isn't specified)
 * sends the files to be includes in the backup
 * closes the backup

Syntax

:   `pukcab backup` [ --[full] ] [ --[name]=_name_ ] [ --[schedule]=_schedule_ ]

### Notes

 * the [name] and [schedule] options are chosen automatically if not specified
 * interrupted backups can be resumed with the [continue] command
 * unless forced, the command will fail if another backup for the same name is already running

`config`
--------

The `config` command displays the current configuration.

Syntax

:   `pukcab config`

`continue`
----------

The `continue` command continues a previously interrupted backup.

Syntax

:   `pukcab continue` [ --[name]=_name_ ] [ --[date]=_date_ ]

### Notes

 * the [name] option is chosen automatically if not specified
 * the [date] option automatically selects the last unfinished backup
 * only unfinished backups may be resumed

`delete`
--------

The `delete` command discards the backup taken at a given [date].

Syntax

:   `pukcab delete` [ --[name]=_name_ ] [ --[date]=_date_ ]

### Notes

 * the [name] option is chosen automatically if not specified
 * the [date] must be specified, unless `--force` is used
 * *all* backups for a given [name] will be deleted if no [date] is specified (`--force` must be used in that case)

`expire`
--------

The `expire` command discards backups following a given [schedule] which are older than a given [age (or date)](#date).

Standard retention schedules have pre-defined retention periods:

:default retention schedules

 schedule   | retention period
------------|------------------
 `daily`    | 2 weeks
 `weekly`   | 6 weeks
 `monthly`  | 365 days
 `yearly`   | 10 years

Syntax

:   `pukcab expire` [ --[name]=_name_ ] [ --[schedule]=_schedule_ ] [ --[age]=_age_ ] [ --[keep]=_keep_ ]

### Notes

 * on the [backup server](#server), the [name] option defaults to all backups if not specified
 * on a [backup client](#client), the [name] option is chosen automatically if not specified
 * the [schedule] and [expiration] are chosen automatically if not specified
 * [schedule] can be a comma-separated list of schedules, in which case any explicit [expiration] will be applied to *all*

`history`
---------

The `history` command shows the different versions stored in backups for given files. Backup sets can be filtered by name and/or date and files.

Syntax

:   `pukcab history` [ --[name]=_name_ ] [ --[date]=_date_ ] [ [_FILES_] ... ]

### Notes

 * if [date] is specified, the command lists only history after that date
 * on server, if [name] is not specified, the command lists all backups, regardless of their name

`info`
------

The `info` command lists the backup sets stored on the server. Backup sets can be filtered by name and/or date and files.

Syntax

:   `pukcab info` [ --[short] ] [ --[name]=_name_ ] [ --[date]=_date_ ] [ [_FILES_] ... ]

### Notes

 * if [date] is specified, the command lists only details about the corresponding backup
 * on server, if [name] is not specified, the command lists all backups, regardless of their name
 * verbose mode lists the individual [files]

`ping`
------

The `ping` command allows to check connectivity to the server.

Syntax

:   `pukcab ping`

### Notes

 * verbose mode displays detailed information during the check

`register`
----------

The `register` command registers a client's SSH public key to the server.

Syntax

:   `pukcab register`

### Notes

 * to register to the backup server, `pukcab` will ask for the dedicated user's password (set on the server)
 * verbose mode displays detailed information during the registration

`restore`
---------

The `restore` command restores [files] as they were at a given [date].

Syntax

:   `pukcab restore` [ --[in-place] ] [ --[directory]=_directory_ ] [ --[name]=_name_ ] [ --[date]=_date_ ] [ [_FILES_] ... ]

### Notes

 * the [name] option is chosen automatically if not specified
 * the [date] option automatically selects the last backup
 * this operation currently requires a working `tar` system command (usually GNU tar)
 * `--in-place` is equivalent to `--directory=/`

`summary`
-----------

The `summary` command lists information about the backup sets stored on the server. Backup sets can be filtered by name.

Syntax

:   `pukcab summary` [ --[name]=_name_ ]

### Notes

 * on server, if [name] is not specified, the command lists all backups, regardless of their name

`vacuum`
--------

The `vacuum` command initiates clean-up of the catalog and vault to save disk space.

Syntax

:   `pukcab vacuum`

### Notes

 * can only be run on the server
 * the clean-up may take a while and delay new backups

`verify`
--------

The verify command reports [files] which have changed since a given [date].

Syntax

:   `pukcab verify` [ --[name]=_name_ ] [ --[date]=_date_ ] [ [_FILES_] ... ]

### Notes

 * the [name] option is chosen automatically if not specified
 * the [date] option automatically selects the last backup if not specified

`web`
-----

The `web` command starts the built-in web interface.

Syntax

:   `pukcab web` [ --[listen]=[_host_]:_port_ ] [ --[root]=_URI_ ]

### Notes

 * by default, `pukcab` listens on `localhost` on port 8080
 * available features depend on the local system's role (client or server)

Options
=======

`pukcab` is quite flexible with the way options are provided:

 * options can be provided in any order
 * options have both a long and a short (1-letter) name (for example, `--name` is `-n`)
 * options can be prefixed with 1 or 2 minus signs (`--option` and `-option` are equivalent)
 * `--option=value` and `--option value` are equivalent (caution: `=` is mandatory for boolean options)

This means that the following lines are all equivalent:

~~~~~~~~~~~~~~~~~~~~~~~~~
pukcab info -n test
pukcab info -n=test
pukcab info --n test
pukcab info --n=test
pukcab info -name test
pukcab info -name=test
pukcab info --name test
pukcab info --name=test
~~~~~~~~~~~~~~~~~~~~~~~~~

General options
---------------

The following options apply to all commands:

option                        description
---------------------------- ------------------------------------------------
`-c`, `--config`[`=`]_file_   specify a [configuration file](#configuration) to use
`-F`, `--force`[`=true`]      ignore non-fatal errors and force action
`-v`, `--verbose`[`=true`]    display more detailed information
`-h`, `--help`                display online help
---------------------------- ------------------------------------------------

`date`
------

Dates are an important concept for `pukcab`.

All backup sets are identified by a unique numeric id and correspond to a set of files at a given point in time (the backup id is actually a [UNIX timestamp](https://en.wikipedia.org/wiki/Unix_time)).
The numeric id can be used to unambiguously specify a given backup set but other, more user-friendly formats are available:

 * a duration in days (default when no unit is specified), hours, minutes is interpreted as relative (in the past, regardless of the actual sign of the duration you specify) to the current time
 * a human-readable date specification in YYYY-MM-DD format is interpreted as absolute (00:00:00 on that date)
 * `now` or `latest` are interpreted as the current time
 * `today` is interpreted as the beginning of the day (local time)

Syntax

:   `--date`[=]*date*

:   `-d` *date*

### Examples

 * `--date 1422577319` means *on the 30th January 2015 at 01:21:59 CET*
 * `--date 0`, `--date now` and `--date latest` mean *now*
 * `--date today` means *today at 00:00*
 * `--date 1` means *yesterday same time*
 * `--date 7` means *last week*
 * `--date -48h` and `--date 48h` both mean *2 days ago*
 * `--date 2h30m` means *2 hours and 30 minutes ago*
 * `--date 2015-01-07` means *on the 7th January 2015 at midnight*

`directory`
-----------

Change to a given directory before restoring entries from the backup.

Syntax

:   `--directory`[=]*directory*

:   `-C` *directory*

Default value

:   _none_ (i.e. extract to the current directory)

`full`
------

Forces a full backup: `pukcab` will send all files to the server, without checking for changes.

Syntax

:   `--full`[`=true`]

:   `--full=false`

:   `-f`

Default value

:   `false`

`in-place`
-----------

Change to the root of the filesystem (`/`) before restoring entries from the backup. This has the effect of restoring the files in-place, overwriting current files.

Syntax

:   `--in-place`[`=true`]

:   `--in-place=false`

:   `--inplace`[`=true`]

:   `--inplace=false`

Default value

:   `false` (i.e. extract to the current directory)

`keep`
------

When expiring data, keep at least a certain number of backups (even if they are expired).

Syntax

:   `--keep`[=]*number*

:   `-k` *number*

Default value

:   `3`

`listen`
--------

Force the built-in web server to listen for connections on a different address/port.

Syntax

:   `--listen`[=][*host*]:*port*

:   `-l` [*host*]:*port*

Default value

:   `localhost:8080`

`name`
------

In `pukcab`, a name is associated with each backup when it's created. It is a free-form text string.

Syntax

:   `--name`[=]*name*

:   `-n` *name*

Default value

:   current host name (output of the `hostname` command)

`schedule`
----------

In `pukcab`, a retention schedule is associated with each backup when it's created and is used when expiring old backups. It is a free-form text string but common values include `daily`, `weekly`, `monthly`, etc.

Syntax

:   `--schedule`[=]*schedule*

:   `-r` *schedule*

Default value

:   (the default value depends on the current day)

:   `daily` from Monday to Saturday

:   `weekly` on Sunday

:   `monthly` on the 1st of the month

:   `yearly` on 1st January

`short`
-------

Display a more concise output.

Syntax

:   `--short`[`=true`]

:   `--short=false`

:   `-s`

Default value

:   `false`

Files
-----

File names can be specified using the usual shell-like wildcards `*` (matches any number of characters) and `?` (matches exactly one character). The following conventions apply:

 * file names starting with a slash ('`/`') are absolute
 * file names not starting with a slash ('`/`') are relative
 * specifying a directory name also selects all the files underneath

### Examples

 * `/home` includes `/home/john`, `/home/dave`, etc. and all the files they contain (i.e. all users' home directories)
 * `*.jpg` includes all `.jpg` files (JPEG images) in all directories
 * `/etc/yum.repos.d/*.repo` includes all repositories configured in Yum
 * `/lib` includes `/lib` and all the files underneath but not `/usr/lib`, `/var/lib`, etc.


Examples
========

(@) Launch a new backup - default options

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
[root@myserver ~]# pukcab backup
[root@myserver ~]#
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(@) Launch a new backup - verbose mode

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
[root@myserver ~]# pukcab backup --verbose
Starting backup: name="myserver" schedule="daily"
Sending file list... done.
New backup: date=1422549975 files=309733
Previous backup: date=1422505656
Determining files to backup... done.
Incremental backup: date=1422549975 files=35
Sending files... done
[root@myserver ~]#
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(@) Verify last backup - default options

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
[root@myserver ~]# pukcab verify
Name:     myserver
Schedule: daily
Date:     1427941113 ( 2015-04-02 04:18:33 +0200 CEST )
Size:     1.6GiB
Files:    50350
Modified: 10
Deleted:  0
Missing:  0
[root@myserver ~]#
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(@) Verify last backup - verbose mode

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
[root@myserver ~]# pukcab verify --verbose
Name:     myserver
Schedule: daily
Date:     1427941113 ( 2015-04-02 04:18:33 +0200 CEST )
m /var/lib/chrony
M /var/lib/chrony/drift
M /var/log/cron
M /var/log/journal/5331b849b3844782aab45e85bd890883/system.journal
M /var/log/journal/5331b849b3844782aab45e85bd890883/user-1001.journal
M /var/log/maillog
M /var/log/messages
M /var/log/secure
M /var/spool/anacron/cron.daily
m /var/tmp
Size:     1.6GiB
Files:    50350
Modified: 10
Deleted:  0
Missing:  0
[root@myserver ~]#
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

(@) Find and recover a deleted file

Let's pretend we want to recover a source RPM for `netatalk` that was deleted a while ago...

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
[root@myserver ~]# pukcab history netatalk\*.src.rpm
1422764854 myserver monthly Sun Feb 1 05:38
-rw-rw-r--  support support   1.8MiB Oct 30 17:30 /home/support/Downloads/netatalk-3.1.6-0.0.4.fc21.src.rpm
-rw-rw-r--  support support   1.7MiB Dec  2 19:59 /home/support/Downloads/netatalk-3.1.7-0.1.fc21.src.rpm

1424572858 myserver weekly Sun Feb 22 03:59

1425177758 myserver monthly Sun Mar 1 03:59

1425782947 myserver weekly Sun Mar 8 04:16

1426387616 myserver weekly Sun Mar 15 04:39

1426817749 myserver daily Fri Mar 20 03:31

1426904886 myserver daily Sat Mar 21 04:24

1426990487 myserver weekly Sun Mar 22 03:28

1427076410 myserver daily Mon Mar 23 04:00

1427165785 myserver daily Tue Mar 24 04:42

1427249388 myserver daily Wed Mar 25 03:24
[root@myserver ~]#
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

We found it! Let's restore it in-place

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
[root@myserver ~]# pukcab restore -d 1422764854 -C / netatalk-3.1.7-0.1.fc21.src.rpm
[root@myserver ~]# cd /home/support/Downloads
[root@myserver ~]# ls -l netatalk-3.1.7-0.1.fc21.src.rpm
-rw-rw-r-- 1 support support 1804762 Dec  2 19:59 netatalk-3.1.7-0.1.fc21.src.rpm
[root@myserver ~]#
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Frequently Asked Questions
==========================

##Client FAQ

Is there a Windows client?

:   No, Windows is currently unsupported. Porting to Windows would require a significant effort and isn't in the plans. This may change, though.

My first backup is taking forever! What should I do?

:   Wait. Another option is to make sure you didn't forget to [exclude] useless or volatile data like
 * big `Downloads` folders
 * caches (Firefox, Safari, Thunderbird tend to keep gigabytes of throw-away data)
 * backups
 * temporary files you forgot to delete

My first backup is *still* taking forever! What should I do?

:   If necessary, you can interrupt it and [continue] later (`pukcab` will think a bit and restart where it was interrupted). Once the first backup is complete, you can take a second one and get rid (with [delete]) of the first one as many files may have changed during such a long backup.

##Server FAQ

Is there a Windows server?

:   No, and there problably never will. If you're seriously considering trusting a Windows server with your most precious data, we humbly suggest reconsidering your backup strategy.

Which filesystem should I use for the catalog? Can I use a network filesystem?

:   No, it must be stored on a local filesystem. You will need a fast filesystem able to deal with potentially big files (several gigabytes are not uncommon) that will be created and removed on the fly. For stability and performance reasons, you should prefer native filesystems, do NOT use NTFS, FAT or [FUSE]-based filesystems.

 * under Linux, you can use [ext3]/[ext4], [XFS], [Btrfs], [ReiserFS], [JFS]
 * under OS X, you can use [HFS+]

Which filesystem should I use for the vault? Can I use a network filesystem?

:   Yes, the vault can be on a remote filesystem. You will need a fast filesystem able to deal with potentially big files and, more importantly, with many files (often thousands and thousands) *in the same directory*. For that reason, do NOT use NTFS, FAT but you can use [FUSE]-based filesystems if they don't put a limit on the number of files.

 * under Linux, you can use [ext3]/[ext4], [XFS], [Btrfs], [ReiserFS], [JFS], [NFS], [SMB], [AFS]...
 * under OS X, you can use [HFS+], [AFP], [NFS], [SMB]...

What are these `catalog.db-wal` and `catalog.db-shm` files? Can I remove them?

:   Short answer: don't, you would lose backups!
:   These files are used to ensure safe concurrent access to the catalog to multiple `pukcab` instances. They are created and deleted automatically, there is no need for you to worry about them.

What is this `catalog.db~` file? Can I remove it?

:   Short answer: you can, but it's not recommended.
:   This file is an automatic backup of your catalog that is created right after each [expire] command. It may therefore be slightly obsolete but it can be used for recovery in case the live catalog gets destroyed/corrupted.

But `catalog.db-wal` is becoming huge! Can I shrink it?

:   Short answer: it will shrink when `pukcab` is done with what it is doing, or after the next [expire] command.
:   Depending of the number and length of concurrent operations, `pukcab` may make heavy use of this working file. You can reduce its maximum size by limiting the number of concurrent operations you run (i.e. you can serialise the [backup] commands).
:   If that doesn't help, you can force-clean it by using [vacuum]
:   If that still doesn't help, make sure no `pukcab` operation is running and issue the following command

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
sqlite3 catalog.db .schema
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

 * replace `catalog.db` by your catalog's path
 * you may need to install [SQLite]
 * this command must be run as the `pukcab` user _on the server_
 * this command will (probably) take very long and block any backup until it has completed

Internals
=========

##Logging

`pukcab` tries to generate useful log records by including easy-to-parse details into the events it sends to [syslog], like in the following examples (from a Linux box):

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Oct 21 08:25:40 server pukcab(newbackup)[16993]: Creating backup set: date=1445391440 name="client" schedule="daily"
Oct 21 08:36:19 client pukcab(backup)[27001]: Could not backup file="/var/log/httpd/access_log" msg="size changed during backup" name="client" date=1445391440 error=warn
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The [syslog] _APP-NAME_ field includes the command that generated the event: `pukcab(`_command_`)`

The rest of the event uses the following format: _message_ [ [_field_=_value_] ... ]

:fields definition

field                type    unit          description
-------------------- ------- ------------  -------------------------------
`date`               number                backup [date]
`duration`           number   seconds      execution time
`elapsed`            number   seconds      total execution time
`error`              token                 error severity (`fatal` / `warn`)
`file`                text                 file name
`files`              number                total number of files
`msg`                 text                 human-readable error message
`name`                text                 backup [name]
`received`           number   bytes        amount of data received
`schedule`            text                 backup [schedule]
`sent`               number   bytes        amount of data sent
`size`               number   bytes        total amount of data
`type`               token                 backup type (`incremental` / `full`)
-------------------- ------- ------------  ----------------


License
=======


                    GNU GENERAL PUBLIC LICENSE
                       Version 2, June 1991

TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION AND MODIFICATION
------------------------------------------------------------------

  0. This License applies to any program or other work which contains
a notice placed by the copyright holder saying it may be distributed
under the terms of this General Public License.  The "Program", below,
refers to any such program or work, and a "work based on the Program"
means either the Program or any derivative work under copyright law:
that is to say, a work containing the Program or a portion of it,
either verbatim or with modifications and/or translated into another
language.  (Hereinafter, translation is included without limitation in
the term "modification".)  Each licensee is addressed as "you".

Activities other than copying, distribution and modification are not
covered by this License; they are outside its scope.  The act of
running the Program is not restricted, and the output from the Program
is covered only if its contents constitute a work based on the
Program (independent of having been made by running the Program).
Whether that is true depends on what the Program does.

  1. You may copy and distribute verbatim copies of the Program's
source code as you receive it, in any medium, provided that you
conspicuously and appropriately publish on each copy an appropriate
copyright notice and disclaimer of warranty; keep intact all the
notices that refer to this License and to the absence of any warranty;
and give any other recipients of the Program a copy of this License
along with the Program.

You may charge a fee for the physical act of transferring a copy, and
you may at your option offer warranty protection in exchange for a fee.

  2. You may modify your copy or copies of the Program or any portion
of it, thus forming a work based on the Program, and copy and
distribute such modifications or work under the terms of Section 1
above, provided that you also meet all of these conditions:

    a) You must cause the modified files to carry prominent notices
    stating that you changed the files and the date of any change.

    b) You must cause any work that you distribute or publish, that in
    whole or in part contains or is derived from the Program or any
    part thereof, to be licensed as a whole at no charge to all third
    parties under the terms of this License.

    c) If the modified program normally reads commands interactively
    when run, you must cause it, when started running for such
    interactive use in the most ordinary way, to print or display an
    announcement including an appropriate copyright notice and a
    notice that there is no warranty (or else, saying that you provide
    a warranty) and that users may redistribute the program under
    these conditions, and telling the user how to view a copy of this
    License.  (Exception: if the Program itself is interactive but
    does not normally print such an announcement, your work based on
    the Program is not required to print an announcement.)

These requirements apply to the modified work as a whole.  If
identifiable sections of that work are not derived from the Program,
and can be reasonably considered independent and separate works in
themselves, then this License, and its terms, do not apply to those
sections when you distribute them as separate works.  But when you
distribute the same sections as part of a whole which is a work based
on the Program, the distribution of the whole must be on the terms of
this License, whose permissions for other licensees extend to the
entire whole, and thus to each and every part regardless of who wrote it.

Thus, it is not the intent of this section to claim rights or contest
your rights to work written entirely by you; rather, the intent is to
exercise the right to control the distribution of derivative or
collective works based on the Program.

In addition, mere aggregation of another work not based on the Program
with the Program (or with a work based on the Program) on a volume of
a storage or distribution medium does not bring the other work under
the scope of this License.

  3. You may copy and distribute the Program (or a work based on it,
under Section 2) in object code or executable form under the terms of
Sections 1 and 2 above provided that you also do one of the following:

    a) Accompany it with the complete corresponding machine-readable
    source code, which must be distributed under the terms of Sections
    1 and 2 above on a medium customarily used for software interchange; or,

    b) Accompany it with a written offer, valid for at least three
    years, to give any third party, for a charge no more than your
    cost of physically performing source distribution, a complete
    machine-readable copy of the corresponding source code, to be
    distributed under the terms of Sections 1 and 2 above on a medium
    customarily used for software interchange; or,

    c) Accompany it with the information you received as to the offer
    to distribute corresponding source code.  (This alternative is
    allowed only for noncommercial distribution and only if you
    received the program in object code or executable form with such
    an offer, in accord with Subsection b above.)

The source code for a work means the preferred form of the work for
making modifications to it.  For an executable work, complete source
code means all the source code for all modules it contains, plus any
associated interface definition files, plus the scripts used to
control compilation and installation of the executable.  However, as a
special exception, the source code distributed need not include
anything that is normally distributed (in either source or binary
form) with the major components (compiler, kernel, and so on) of the
operating system on which the executable runs, unless that component
itself accompanies the executable.

If distribution of executable or object code is made by offering
access to copy from a designated place, then offering equivalent
access to copy the source code from the same place counts as
distribution of the source code, even though third parties are not
compelled to copy the source along with the object code.

  4. You may not copy, modify, sublicense, or distribute the Program
except as expressly provided under this License.  Any attempt
otherwise to copy, modify, sublicense or distribute the Program is
void, and will automatically terminate your rights under this License.
However, parties who have received copies, or rights, from you under
this License will not have their licenses terminated so long as such
parties remain in full compliance.

  5. You are not required to accept this License, since you have not
signed it.  However, nothing else grants you permission to modify or
distribute the Program or its derivative works.  These actions are
prohibited by law if you do not accept this License.  Therefore, by
modifying or distributing the Program (or any work based on the
Program), you indicate your acceptance of this License to do so, and
all its terms and conditions for copying, distributing or modifying
the Program or works based on it.

  6. Each time you redistribute the Program (or any work based on the
Program), the recipient automatically receives a license from the
original licensor to copy, distribute or modify the Program subject to
these terms and conditions.  You may not impose any further
restrictions on the recipients' exercise of the rights granted herein.
You are not responsible for enforcing compliance by third parties to
this License.

  7. If, as a consequence of a court judgment or allegation of patent
infringement or for any other reason (not limited to patent issues),
conditions are imposed on you (whether by court order, agreement or
otherwise) that contradict the conditions of this License, they do not
excuse you from the conditions of this License.  If you cannot
distribute so as to satisfy simultaneously your obligations under this
License and any other pertinent obligations, then as a consequence you
may not distribute the Program at all.  For example, if a patent
license would not permit royalty-free redistribution of the Program by
all those who receive copies directly or indirectly through you, then
the only way you could satisfy both it and this License would be to
refrain entirely from distribution of the Program.

If any portion of this section is held invalid or unenforceable under
any particular circumstance, the balance of the section is intended to
apply and the section as a whole is intended to apply in other
circumstances.

It is not the purpose of this section to induce you to infringe any
patents or other property right claims or to contest validity of any
such claims; this section has the sole purpose of protecting the
integrity of the free software distribution system, which is
implemented by public license practices.  Many people have made
generous contributions to the wide range of software distributed
through that system in reliance on consistent application of that
system; it is up to the author/donor to decide if he or she is willing
to distribute software through any other system and a licensee cannot
impose that choice.

This section is intended to make thoroughly clear what is believed to
be a consequence of the rest of this License.

  8. If the distribution and/or use of the Program is restricted in
certain countries either by patents or by copyrighted interfaces, the
original copyright holder who places the Program under this License
may add an explicit geographical distribution limitation excluding
those countries, so that distribution is permitted only in or among
countries not thus excluded.  In such case, this License incorporates
the limitation as if written in the body of this License.

  9. The Free Software Foundation may publish revised and/or new versions
of the General Public License from time to time.  Such new versions will
be similar in spirit to the present version, but may differ in detail to
address new problems or concerns.

Each version is given a distinguishing version number.  If the Program
specifies a version number of this License which applies to it and "any
later version", you have the option of following the terms and conditions
either of that version or of any later version published by the Free
Software Foundation.  If the Program does not specify a version number of
this License, you may choose any version ever published by the Free Software
Foundation.

  10. If you wish to incorporate parts of the Program into other free
programs whose distribution conditions are different, write to the author
to ask for permission.  For software which is copyrighted by the Free
Software Foundation, write to the Free Software Foundation; we sometimes
make exceptions for this.  Our decision will be guided by the two goals
of preserving the free status of all derivatives of our free software and
of promoting the sharing and reuse of software generally.

NO WARRANTY
-----------

  11. BECAUSE THE PROGRAM IS LICENSED FREE OF CHARGE, THERE IS NO WARRANTY
FOR THE PROGRAM, TO THE EXTENT PERMITTED BY APPLICABLE LAW.  EXCEPT WHEN
OTHERWISE STATED IN WRITING THE COPYRIGHT HOLDERS AND/OR OTHER PARTIES
PROVIDE THE PROGRAM "AS IS" WITHOUT WARRANTY OF ANY KIND, EITHER EXPRESSED
OR IMPLIED, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE.  THE ENTIRE RISK AS
TO THE QUALITY AND PERFORMANCE OF THE PROGRAM IS WITH YOU.  SHOULD THE
PROGRAM PROVE DEFECTIVE, YOU ASSUME THE COST OF ALL NECESSARY SERVICING,
REPAIR OR CORRECTION.

  12. IN NO EVENT UNLESS REQUIRED BY APPLICABLE LAW OR AGREED TO IN WRITING
WILL ANY COPYRIGHT HOLDER, OR ANY OTHER PARTY WHO MAY MODIFY AND/OR
REDISTRIBUTE THE PROGRAM AS PERMITTED ABOVE, BE LIABLE TO YOU FOR DAMAGES,
INCLUDING ANY GENERAL, SPECIAL, INCIDENTAL OR CONSEQUENTIAL DAMAGES ARISING
OUT OF THE USE OR INABILITY TO USE THE PROGRAM (INCLUDING BUT NOT LIMITED
TO LOSS OF DATA OR DATA BEING RENDERED INACCURATE OR LOSSES SUSTAINED BY
YOU OR THIRD PARTIES OR A FAILURE OF THE PROGRAM TO OPERATE WITH ANY OTHER
PROGRAMS), EVEN IF SUCH HOLDER OR OTHER PARTY HAS BEEN ADVISED OF THE
POSSIBILITY OF SUCH DAMAGES.

                     END OF TERMS AND CONDITIONS

[_OPTIONS_]: #options
[_FILES_]: #files
[backup]: #backup
[continue]: #continue
[resume]: #continue
[save]: #backup
[restore]: #restore
[verify]: #verify
[check]: #verify
[delete]: #delete
[purge]: #delete
[expire]: #expire
[vacuum]: #vacuum
[config]: #config
[cfg]: #config
[info]: #info
[history]: #history
[versions]: #history
[list]: #info
[ping]: #ping
[test]: #ping
[register]: #register
[web]: #web
[dashboard]: #summary
[summary]: #summary

[name]: #name
[date]: #date
[schedule]: #schedule
[full]: #full
[short]: #short
[keep]: #keep
[files]: #files
[age]: #date
[expiration]: #date
[listen]: #listen
[directory]: #directory
[in-place]: #in-place

[create SSH keys]: https://en.wikipedia.org/wiki/Ssh-keygen
[OS-dependent]: #os-dependent-defaults
[exclude]: #including-excluding-items
[HFS+]: https://support.apple.com/en-us/HT201711
[Btrfs]: https://en.wikipedia.org/wiki/Btrfs
[ext3]: https://en.wikipedia.org/wiki/Ext3
[ext4]: https://en.wikipedia.org/wiki/Ext4
[ReiserFS]: https://en.wikipedia.org/wiki/ReiserFS
[XFS]: https://en.wikipedia.org/wiki/XFS
[JFS]: https://en.wikipedia.org/wiki/JFS_%28file_system%29
[AFS]: https://en.wikipedia.org/wiki/Andrew_File_System
[AFP]: https://en.wikipedia.org/wiki/Apple_Filing_Protocol
[SMB]: https://en.wikipedia.org/wiki/Server_Message_Block
[NFS]: https://en.wikipedia.org/wiki/Network_File_System
[FUSE]: https://en.wikipedia.org/wiki/Filesystem_in_Userspace
[SQLite]: http://www.sqlite.org/
[BusyBox]: http://www.busybox.net/
[syslog]: https://tools.ietf.org/html/rfc5424
