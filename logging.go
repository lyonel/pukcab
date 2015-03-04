package main

import (
	"io/ioutil"
	"log"
	"os"
)

const debugFlags = log.LstdFlags | log.Lshortfile
const debugPrefix = "[DEBUG] "
const failPrefix = "[ERROR] "

var debug = log.New(ioutil.Discard, "", 0)
var info = log.New(ioutil.Discard, "", 0)
var failure = log.New(os.Stderr, failPrefix, 0)

func Debug(on bool) {
	if on {
		debug = log.New(os.Stderr, debugPrefix, debugFlags)
	} else {
		debug = log.New(ioutil.Discard, "", 0)
	}
}

func Info(on bool) {
	if on {
		info = log.New(os.Stdout, "", 0)
	} else {
		info = log.New(ioutil.Discard, "", 0)
	}
}

func Failure(on bool) {
	if on {
		failure = log.New(os.Stderr, failPrefix, 0)
	} else {
		failure = log.New(ioutil.Discard, "", 0)
	}
}
