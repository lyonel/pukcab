package main

import ()

type Backup struct {
	Date           BackupID
	Name, Schedule string

	backupset   map[string]struct{}
	directories map[string]bool

	include, exclude []string
}

func NewBackup(cfg Config) *Backup {
	return &Backup{
		backupset:   make(map[string]struct{}),
		directories: make(map[string]bool),

		include: cfg.Include,
		exclude: cfg.Exclude,
	}
}

func (b *Backup) Start(name string, schedule string) {
	b.Name, b.Schedule = name, schedule

}

func (b *Backup) Count() int {
	return len(b.backupset)
}
