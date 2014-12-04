package main

import (
	"fmt"
	"strconv"
	"time"
)

type SQLInt int64

func (i *SQLInt) Scan(src interface{}) error {
	*i = 0
	switch s := src.(type) {
	case int64:
		*i = SQLInt(s)
	case float64:
		*i = SQLInt(s)
	case bool:
		if s {
			*i = 1
		} else {
			*i = 0
		}
	case time.Time:
		*i = SQLInt(s.Unix())
	case string:
		if v, err := strconv.ParseInt(s, 0, 64); err == nil {
			*i = SQLInt(v)
		}
	}
	return nil
}

type SQLString string

func (i *SQLString) Scan(src interface{}) error {
	if src == nil {
		*i = ""
	} else {
		*i = SQLString(fmt.Sprintf("%v", src))
	}
	return nil
}
