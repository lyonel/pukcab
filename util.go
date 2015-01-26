package main

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BackupID int64

type BackupInfo struct {
	Date           BackupID
	Finished       time.Time
	Name, Schedule string
	Files, Size    int64
}

func (id *BackupID) String() string {
	return fmt.Sprintf("%d", *id)
}

func (id *BackupID) Set(s string) error {
	// number (positive or negative) suffixed with 'd': go back by n days
	if matched, err := regexp.MatchString("^-?[0-9]+d$", s); err == nil && matched {
		s = s[:len(s)-1]
	}

	// "small" number (positive or negative): go back by n days
	// "big" number (positive): interpret as a UNIX epoch-based time (i.e. a backup id)
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

	// go back by a given duration (for example 48h or 120m)
	if duration, err := time.ParseDuration(s); err == nil {
		*id = BackupID(time.Now().Unix() - int64(math.Abs(duration.Seconds())))
		return nil
	}

	// go back to a given date
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

func Hash(filename string) (hash string) {
	if file, err := os.Open(filename); err == nil {
		checksum := sha512.New()
		if _, err = io.Copy(checksum, file); err == nil {
			hash = EncodeHash(checksum.Sum(nil))
		}
		file.Close()
	}

	return
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
