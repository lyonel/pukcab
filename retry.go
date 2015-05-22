package main

import (
	"math/rand"
	"time"
)

type Operation func() error

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
