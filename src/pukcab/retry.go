package main

import (
	"math/rand"
	"time"
)

type Operation func() error
type Condition func(error) bool

func retry(n int, o Operation) (err error) {
	for ; n > 0; n-- {
		if err = o(); err == nil {
			return
		}
		// sleep between 1 and 2 seconds
		time.Sleep(time.Second + time.Duration(rand.Int63n(int64(time.Second))))
	}

	return err
}

func retryif(n int, c Condition, o Operation) (err error) {
	for ; n > 0; n-- {
		if err = o(); err == nil || !c(err) {
			return err
		}
		// sleep between 1 and 2 seconds
		time.Sleep(time.Second + time.Duration(rand.Int63n(int64(time.Second))))
	}

	return err
}
