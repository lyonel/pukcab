package main

import (
	"math/rand"
	"time"
)

type operation func() error
type condition func(error) bool

func retry(n int, o operation) (err error) {
	for ; n > 0; n-- {
		if err = o(); err == nil {
			return
		}
		// sleep between 1 and 2 seconds
		time.Sleep(time.Second + time.Duration(rand.Int63n(int64(time.Second))))
	}

	return err
}

func retryif(n int, c condition, o operation) (err error) {
	for ; n > 0; n-- {
		if err = o(); err == nil || !c(err) {
			return err
		}
		// sleep between 1 and 2 seconds
		time.Sleep(time.Second + time.Duration(rand.Int63n(int64(time.Second))))
	}

	return err
}
