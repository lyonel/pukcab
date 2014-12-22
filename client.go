package main

import (
	"archive/tar"
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/antage/mntent"
)

var directories map[string]bool
var backupset map[string]struct{}

func expire() {
	flag.Int64Var(&date, "date", date, "Backup set")
	flag.Int64Var(&date, "d", date, "-date")
	flag.UintVar(&age, "age", age, "Age")
	flag.UintVar(&age, "a", age, "-age")
	flag.Parse()

	log.Println("name: ", name)
	log.Println("date: ", date)
	log.Println("schedule: ", schedule)
	log.Println("age: ", age)
}

func contains(set []string, e string) bool {
	for _, a := range set {
		if a == e {
			return true
		}

		if filepath.IsAbs(a) {
			if strings.HasPrefix(e, a+string(filepath.Separator)) {
				return true
			}
		} else {
			if matched, _ := filepath.Match(a, filepath.Base(e)); matched {
				return true
			}

			if strings.HasPrefix(a, "."+string(filepath.Separator)) {
				if _, err := os.Lstat(filepath.Join(e, a)); !os.IsNotExist(err) {
					return true
				}
			}
		}
	}
	return false
}

func includeorexclude(e *mntent.Entry) bool {
	result := !(contains(cfg.Exclude, e.Types[0]) || contains(cfg.Exclude, e.Directory)) && (contains(cfg.Include, e.Types[0]) || contains(cfg.Include, e.Directory))

	directories[e.Directory] = result
	return result
}

func excluded(f string) bool {
	if _, known := directories[f]; known {
		return !directories[f]
	}
	return contains(cfg.Exclude, f) && !contains(cfg.Include, f)
}

func addfiles(d string) {
	backupset[d] = struct{}{}
	files, _ := ioutil.ReadDir(d)
	for _, f := range files {
		file := filepath.Join(d, f.Name())

		if f.Mode()&os.ModeTemporary != os.ModeTemporary {
			backupset[file] = struct{}{}

			if f.IsDir() && !excluded(file) {
				addfiles(file)
			}
		}
	}
}

func backup() {
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
	flag.BoolVar(&full, "full", full, "Full backup")
	flag.BoolVar(&full, "f", full, "-full")
	flag.Parse()

	log.Printf("Starting backup: name=%q schedule=%q\n", name, schedule)

	directories = make(map[string]bool)
	backupset = make(map[string]struct{})
	devices := make(map[string]bool)

	if mtab, err := mntent.Parse("/etc/mtab"); err != nil {
		log.Println("Failed to parse /etc/mtab: ", err)
	} else {
		for i := range mtab {
			if !devices[mtab[i].Name] && includeorexclude(mtab[i]) {
				devices[mtab[i].Name] = true
			}
		}
	}

	for i := range cfg.Include {
		if filepath.IsAbs(cfg.Include[i]) {
			directories[cfg.Include[i]] = true
		}
	}

	for d := range directories {
		if directories[d] {
			addfiles(d)
		}
	}

	cmd := remotecommand("newbackup", "-name", name, "-schedule", schedule)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	for f := range backupset {
		fmt.Fprintln(stdin, f)
	}
	stdin.Close()

	date = 0
	var previous int64 = 0
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		if date, err = strconv.ParseInt(scanner.Text(), 10, 0); err != nil {
			fmt.Println("Protocol error")
			log.Fatal("Protocol error")
		}
	}

	if date == 0 {
		scanner.Scan()
		errmsg := scanner.Text()
		fmt.Println("Server error:", errmsg)
		log.Fatal("Server error:", errmsg)
	} else {
		log.Printf("New backup: date=%d files=%d\n", date, len(backupset))
		if scanner.Scan() {
			previous, _ = strconv.ParseInt(scanner.Text(), 10, 0)
			if previous > 0 {
				log.Printf("Previous backup: date=%d\n", previous)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func humanateBytes(s uint64, base float64, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%dB", s)
	}
	e := math.Floor(logn(float64(s), base))
	suffix := sizes[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f%s"
	if val < 10 {
		f = "%.1f%s"
	}
	return fmt.Sprintf(f, val, suffix)
}

// Bytes produces a human readable representation of an byte size.
func Bytes(s uint64) string {
	sizes := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	return humanateBytes(s, 1024, sizes)
}

func info() {
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Int64Var(&date, "date", 0, "Backup set")
	flag.Int64Var(&date, "d", 0, "-date")
	flag.Parse()

	var cmd *exec.Cmd
	if date != 0 {
		cmd = remotecommand("backupinfo", "-date", fmt.Sprintf("%d", date))
	} else {
		cmd = remotecommand("backupinfo", "-name", name)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	tr := tar.NewReader(stdout)
	size := int64(0)
	files := int64(0)
	missing := int64(0)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			size = 0
			files = 0
			missing = 0
			fmt.Printf("\nName: %s\nSchedule: %s\nDate: %d (%v)\n", hdr.Name, hdr.Linkname, hdr.ModTime.Unix(), hdr.ModTime)
		default:
			files++
			if s, err := strconv.ParseInt(hdr.Xattrs["backup.size"], 0, 0); err == nil {
				size += s
			}
			if hdr.Xattrs["backup.type"] == "?" {
				missing++
			}
		}
	}
	fmt.Printf("Files: %d\nSize: %s\n", files, Bytes(uint64(size)))
	fmt.Printf("Complete: ")
	if files > 0 && missing > 0 {
		fmt.Printf("%.1f%% (%d files missing)\n", 100*float64(files-missing)/float64(files), missing)
	} else {
		fmt.Println("yes")
	}

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}
