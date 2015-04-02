# Pukcab

## Introduction

`pukcab` is a lightweight, single-binary backup system for UNIX / Linux systems that stores de-duplicated, compressed and incremental backups on a remote server using just an SSH connection.

De-duplication happens not only between incremental backups of the same system but also between different systems. For example, it allows you to perform full backups of systems running the same OS with only minimal disk space for each additional system^[Basically, only configuration files and user data require disk space.].

## Intended use

`pukcab` doesn't compare to professional-grade backup systems, *don't expect it to be able to backup thousands of systems or dozens of terabytes of data*.

It is, however, perfectly suitable for home users, hobbyists, UNIX / Linux enthusiasts or small tech-savy shops who want a flexible, yet dead-easy to setup and operate backup system with no bigger expenses than just a Linux box with a few terabytes of storage.

Little to no configuration should ever be required to satisfy most needs:

 * just run `pukcab backup` every night on your systems
 * full-system backups should be preferred, thanks to sensible OS-dependent default exclusions
 * automatic daily/weekly/monthly/yearly retention policies should keep enough backups around

## Features

 * lightweight (just [#Download 1 binary] to be installed on both the client and the server)
 * easy to install (only 1 username with SSH connectivity is required to set up a server)
 * flexible configuration
 * sensible defaults
 * automatic retention schedules
 * incremental/full backups
 * data de-duplication
 * data compression
 * (optional) web interface

## Documentation

 * [User's Manual](http://ezix.org/project/raw-attachment/wiki/Pukcab/MANUAL.pdf)
 * [HTML version](http://ezix.org/software/files/Pukcab-MANUAL.html)

## Download

 * [all platforms](http://ezix.org/download/?package=pukcab.ezix.org) (source code -- you will need a [Go](http://golang.org Go) environment to compile)
 * [Linux for ARM](http://ezix.org/download/?package=arm.linux.pukcab.ezix.org) (Raspberry Pi and the like)
 * [Linux for x86-64](http://ezix.org/download/?package=x86-64.linux.pukcab.ezix.org) (Fedora, Debian, Ubuntu, Red Hat, ...)
 * [Linux for i686 (32bit)](http://ezix.org/download/?package=i686.linux.pukcab.ezix.org) (Fedora, Debian, Ubuntu, Red Hat, ...)
 * [Linux RPMs](http://ezix.org/download/?package=rpm.pukcab.ezix.org) (Fedora, CentOS, Red Hat, ...)
 * [Mac OS X (64bit)]http://ezix.org/download/?package=osx.pukcab.ezix.org) (Mavericks, Yosemite, ...) -- **beta**




