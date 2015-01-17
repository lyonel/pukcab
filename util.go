package main

import (
	"encoding/base64"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BackupID int64

func (id *BackupID) String() string {
	return fmt.Sprintf("%d", *id)
}

func (id *BackupID) Set(s string) error {
	if i, err := strconv.ParseInt(s, 10, 0); err == nil {
		if i > 36524 {
			*id = BackupID(i)
		} else {
			// if a value is small enough, interpret it as a number of days (at most 100 years of approx 365.2425 days)
			if i < 0 {
				i = -i
			}
			*id = BackupID(time.Now().Unix() - i*24*60*60)
		}
		return nil
	}

	if duration, err := time.ParseDuration(s); err == nil {
		*id = BackupID(time.Now().Unix() - int64(math.Abs(duration.Seconds())))
		return nil
	}

	if date, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		*id = BackupID(date.Unix())
		return nil
	} else {
		return err
	}
}

func EncodeHash(h []byte) (hash string) {
	hash = base64.StdEncoding.EncodeToString(h)
	hash = strings.Replace(hash, "/", "_", -1)
	hash = strings.Replace(hash, "+", "-", -1)
	hash = strings.Trim(hash, "=")

	return hash
}

func ConvertGlob(filter string) (regex string) {
	regex = regexp.QuoteMeta(strings.TrimRight(filter, string(filepath.Separator)))
	regex = strings.Replace(regex, "\\?", ".", -1)
	regex = strings.Replace(regex, "\\*", ".*", -1)

	if len(regex) > 0 && regex[0] == filepath.Separator {
		regex = "^" + regex
	} else {
		regex = string(filepath.Separator) + regex
	}
	regex = regex + "(" + string(filepath.Separator) + ".*)?$"

	return
}
