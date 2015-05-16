package main

import (
	"os"
	"time"
)

const programName = "pukcab"
const versionMajor = 1
const versionMinor = 5
const defaultCommand = "help"
const defaultCatalog = "catalog.db"
const defaultVault = "vault"
const defaultMaxtries = 10
const defaultTimeout = 6 * 3600 // 6 hours

const protocolVersion = 1

var programFile string = "backup"
var defaultName string = "backup"
var defaultSchedule string = "daily"

var verbose bool = false
var protocol int = protocolVersion
var timeout int = defaultTimeout

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

}
