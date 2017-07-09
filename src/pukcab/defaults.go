package main

import (
	"os"
	"runtime"
	"time"
)

var buildID string

const programName = "pukcab"
const versionMajor = 1
const versionMinor = 5
const defaultCommand = "help"
const defaultCatalog = "catalog.db"
const defaultVault = "vault"
const defaultMaxtries = 10
const defaultTimeout = 6 * 3600 // 6 hours

const protocolVersion = 1

var programFile = "backup"
var defaultName = "backup"
var defaultSchedule = "daily"

var verbose = false
var protocol = protocolVersion
var timeout = defaultTimeout
var force = false

func setDefaults() {
	defaultName, _ = os.Hostname()

	now := time.Now()
	if now.Weekday() == time.Sunday {
		defaultSchedule = "weekly"
	}
	if now.Day() == 1 {
		defaultSchedule = "monthly"
	}
	if now.Day() == 1 && now.Month() == 1 {
		defaultSchedule = "yearly"
	}

	if int(LoadAvg()) > runtime.NumCPU() {
		Renice()
	}
}
