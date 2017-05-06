package main

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const infinite = -1

type BackupID int64

type BackupInfo struct {
	Date           BackupID
	Finished       time.Time
	LastModified   time.Time
	Name, Schedule string
	Files, Size    int64
}

func (id *BackupID) String() string {
	return fmt.Sprintf("%d", *id)
}

func (id *BackupID) Time() time.Time {
	return time.Unix(int64(*id), 0)
}

func (id *BackupID) Set(s string) error {
	switch strings.ToLower(s) {
	case "now", "latest", "last":
		*id = BackupID(time.Now().Unix())
		return nil

	case "today":
		y, m, d := time.Now().Date()
		*id = BackupID(time.Date(y, m, d, 0, 0, 0, 0, time.Local).Unix())
		return nil
	}

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

func Hash(filename string) (hash1 string, hash2 string) {
	if file, err := os.Open(filename); err == nil {
		defer file.Close()

		if fileinfo, err := file.Stat(); err == nil {
			h1 := sha512.New()
			h2 := sha1.New()
			io.WriteString(h2, "blob "+strconv.FormatInt(fileinfo.Size(), 10)+"\000")

			if _, err := io.Copy(io.MultiWriter(h1, h2), file); err == nil {
				return EncodeHash(h1.Sum(nil)), hex.EncodeToString(h2.Sum(nil))
			}
		}
	}

	return
}

func ConvertGlob(name string, depth int, filters ...string) (SQL string) {
	clauses := []string{}

	for _, f := range filters {
		f = strings.TrimRight(f, string(filepath.Separator))
		if f == "" {
			f = "/"
		}
		f = strings.Replace(f, "'", "''", -1)
		if !filepath.IsAbs(f) {
			f = filepath.Join("*", f)
		}
		clauses = append(clauses, name+" GLOB '"+f+"'")
		if depth != 0 && filepath.Base(f) != "*" {
			if depth == 1 {
				clauses = append(clauses, "("+name+" GLOB '"+filepath.Join(f, "*")+"' AND NOT "+name+" GLOB '"+filepath.Join(f, "*", "*")+"')")
			} else {
				clauses = append(clauses, name+" GLOB '"+filepath.Join(f, "*")+"'")
			}
		}
	}

	return strings.Join(clauses, " OR ")
}

func human(s uint64, base float32, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%d%s", s, sizes[0])
	}
	val := float32(s)
	suffix := 0

	for val >= base {
		val = val / base
		suffix++
	}
	f := "%.0f%s"
	if val < 10 {
		f = "%.1f%s"
	}
	return fmt.Sprintf(f, val, sizes[suffix])
}

// Bytes produces a human readable representation of an byte size.
func Bytes(s uint64) string {
	sizes := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	return human(s, 1024, sizes)
}

func printdebug() {
	_, fn, line, _ := runtime.Caller(1)
	log.Printf("DEBUG %s:%d\n", fn, line)
}

func DisplayTime(d time.Time) string {
	if time.Since(d).Hours() < 365*24 {
		return d.Format("Jan _2 15:04")
	} else {
		return d.Format("Jan _2  2006")
	}
}

func Exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
