package main

import (
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
)

const debugFlags = log.Lshortfile
const debugPrefix = "[DEBUG] "
const failPrefix = "[ERROR] "

var debug = log.New(ioutil.Discard, "", 0)
var info = log.New(ioutil.Discard, "", 0)
var failure = log.New(os.Stderr, failPrefix, 0)

// Debug enables (or disables) debug logging
func Debug(on bool) {
	if on {
		arg0 := os.Args[0]
		os.Args[0] = programFile
		defer func() { os.Args[0] = arg0 }()

		var err error
		if debug, err = syslog.NewLogger(syslog.LOG_NOTICE|syslog.LOG_USER, debugFlags); err == nil {
			debug.SetPrefix(debugPrefix)
			return
		}
	}
	debug = log.New(ioutil.Discard, "", 0)
}

// Info enables (or disables) info logging
func Info(on bool) {
	if on {
		info = log.New(os.Stdout, "", 0)
	} else {
		info = log.New(ioutil.Discard, "", 0)
	}
}

// Failure enables (or disables) error logging
func Failure(on bool) {
	if on {
		failure = log.New(os.Stderr, failPrefix, 0)
	} else {
		failure = log.New(ioutil.Discard, "", 0)
	}
}

// LogStream represents a bespoke logger
type LogStream struct {
	*log.Logger
}

// NewLogStream creates a new bespoke logger
func NewLogStream(logger *log.Logger) (stream *LogStream) {
	stream = &LogStream{
		Logger: logger,
	}
	return
}

func (l *LogStream) Write(p []byte) (n int, err error) {
	l.Printf("%s", p)
	return len(p), nil
}

// LogExit logs an error and exits (returning 2 for EBUSY and 1 otherwise)
func LogExit(err error) {
	if busy(err) {
		os.Exit(2)
	} else {
		log.Printf("Exiting: name=%q date=%d msg=%q error=fatal\n", name, date, err)
		os.Exit(1)
	}
}
