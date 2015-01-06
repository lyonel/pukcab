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
	"time"

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

		if !IsNodump(f, file) {
			backupset[file] = struct{}{}

			if f.IsDir() && !excluded(file) {
				addfiles(file)
			}
		}
	}
}

func backup() {
	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
	flag.BoolVar(&full, "full", full, "Full backup")
	flag.BoolVar(&full, "f", full, "-full")
	flag.Parse()

	log.Printf("Starting backup: name=%q schedule=%q\n", name, schedule)
	if verbose {
		fmt.Printf("Starting backup: name=%q schedule=%q\n", name, schedule)
	}

	directories = make(map[string]bool)
	backupset = make(map[string]struct{})
	devices := make(map[string]bool)

	if mtab, err := mntent.Parse("/etc/mtab"); err != nil {
		log.Println("Failed to parse /etc/mtab: ", err)
	} else {
		for _, m := range mtab {
			if !devices[m.Name] && includeorexclude(m) {
				devices[m.Name] = true
			}
		}
	}

	for _, i := range cfg.Include {
		if filepath.IsAbs(i) {
			directories[i] = true
		}
	}

	for d := range directories {
		if directories[d] {
			addfiles(d)
		}
	}

	if verbose {
		fmt.Print("Sending file list... ")
	}

	cmd := remotecommand("newbackup", "-name", name, "-schedule", schedule, "-full", strconv.FormatBool(full))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	for f := range backupset {
		fmt.Fprintln(stdin, f)
	}
	stdin.Close()

	if verbose {
		fmt.Println("done.")
	}

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
		if verbose {
			fmt.Printf("New backup: date=%d files=%d\n", date, len(backupset))
		}
		log.Printf("New backup: date=%d files=%d\n", date, len(backupset))
		if scanner.Scan() {
			previous, _ = strconv.ParseInt(scanner.Text(), 10, 0)
			if previous > 0 {
				if verbose {
					fmt.Printf("Previous backup: date=%d\n", previous)
				}
				log.Printf("Previous backup: date=%d\n", previous)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if verbose {
		fmt.Print("Sending files... ")
	}

	cmd = remotecommand("submitfiles", "-name", name, "-date", fmt.Sprintf("%d", date))
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
	stdin, err = cmd.StdinPipe()
	if err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	tw := tar.NewWriter(stdin)
	defer tw.Close()

	globaldata := paxHeaders(map[string]interface{}{
		".name":     name,
		".schedule": schedule,
		".version":  fmt.Sprintf("%d.%d", versionMajor, versionMinor),
	})
	globalhdr := &tar.Header{
		Name:     name,
		Size:     int64(len(globaldata)),
		Linkname: schedule,
		ModTime:  time.Unix(date, 0),
		Typeflag: tar.TypeXGlobalHeader,
	}
	tw.WriteHeader(globalhdr)
	tw.Write(globaldata)

	for f := range backupset {
		if fi, err := os.Lstat(f); err != nil {
			log.Println(err)
		} else {
			if hdr, err := tar.FileInfoHeader(fi, ""); err == nil {
				hdr.Uname = Username(hdr.Uid)
				hdr.Gname = Groupname(hdr.Gid)
				hdr.Name = f
				if fi.Mode()&os.ModeSymlink != 0 {
					hdr.Linkname, _ = os.Readlink(f)
				}
				if fi.Mode()&os.ModeDevice != 0 {
					hdr.Devmajor, hdr.Devminor = DevMajorMinor(f)
				}
				if !fi.Mode().IsRegular() {
					hdr.Size = 0
				}
				attributes := Attributes(f)
				if len(attributes) > 0 {
					hdr.Xattrs = make(map[string]string)
					for _, a := range attributes {
						hdr.Xattrs[a] = string(Attribute(f, a))
					}
				}
				if fi.Mode().IsRegular() {
					if file, err := os.Open(f); err != nil {
						log.Println(err)
					} else {
						var written int64 = 0
						buf := make([]byte, 1024*1024) // 1MiB

						tw.WriteHeader(hdr)
						for {
							nr, er := file.Read(buf)
							if er == io.EOF {
								break
							}
							if er != nil {
								log.Fatal("Could not read ", f, ": ", er)
							}
							if nr > 0 {
								nw, ew := tw.Write(buf[0:nr])
								if ew != nil {
									if ew == tar.ErrWriteTooLong {
										break
									}
									log.Fatal("Could not send ", f, ": ", ew)
								} else {
									written += int64(nw)
								}
							}
						}
						file.Close()

						if written != hdr.Size {
							log.Println("Could not backup ", f, ":", hdr.Size, " bytes expected but ", written, " bytes written")
						}
					}
				} else {
					tw.WriteHeader(hdr)
				}
			} else {
				log.Printf("Couldn't backup %s: %s\n", f, err)
			}
		}
	}

	stdin.Close()

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if verbose {
		fmt.Println("done.")
	}

}

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func human(s uint64, base float64, sizes []string) string {
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
	sizes := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	return human(s, 1024, sizes)
}

func info() {
	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Int64Var(&date, "date", 0, "Backup set")
	flag.Int64Var(&date, "d", 0, "-date")
	flag.Parse()

	var cmd *exec.Cmd
	if date != 0 {
		cmd = remotecommand("backupinfo", "-date", fmt.Sprintf("%d", date))
	} else {
		if name != "" {
			cmd = remotecommand("backupinfo", "-name", name)
		} else {
			cmd = remotecommand("backupinfo")
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
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
			fmt.Println("Backend error:", err)
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
			if verbose {
				fmt.Printf("%s %8s %-8s", hdr.FileInfo().Mode(), hdr.Uname, hdr.Gname)
				if s, err := strconv.ParseUint(hdr.Xattrs["backup.size"], 0, 0); err == nil {
					fmt.Printf("%10s", Bytes(s))
				} else {
					fmt.Printf("%10s", "")
				}
				fmt.Printf(" %s", hdr.Name)
				if hdr.Linkname != "." {
					fmt.Printf(" ➙ %s\n", hdr.Linkname)
				} else {
					fmt.Println()
				}
			}
		}
	}
	if files > 0 {
		fmt.Printf("Files: %d\nSize: %s\n", files, Bytes(uint64(size)))
		fmt.Printf("Complete: ")
		if files > 0 && missing > 0 {
			fmt.Printf("%.1f%% (%d files missing)\n", 100*float64(files-missing)/float64(files), missing)
		} else {
			fmt.Println("yes")
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
}
