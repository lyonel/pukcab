package main

import (
	"bufio"
	"compress/gzip"
	"encoding/gob"
	"ezix.org/tar"
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

type Status int

const (
	OK = iota
	Modified
	MetaModified
	Deleted
	Missing
	Unknown
)

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
			if matched, _ := filepath.Match(a, filepath.Base(e)); matched && strings.ContainsAny(a, "*?[") {
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

	if contains(cfg.Exclude, d) {
		return
	}

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

	if mtab, err := loadmtab(); err != nil {
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

	cmd := remotecommand("newbackup", "-name", name, "-schedule", schedule, "-full="+strconv.FormatBool(full))
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
		if d, err := strconv.ParseInt(scanner.Text(), 10, 0); err != nil {
			fmt.Println("Protocol error")
			log.Fatal("Protocol error")
		} else {
			date = BackupID(d)
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

	if !full {
		cmd = remotecommand("metadata", "-name", name, "-date", fmt.Sprintf("%d", date))

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
				if verbose {
					fmt.Print("Determining files to backup... ")
				}
			default:
				if len(hdr.Xattrs["backup.type"]) > 0 {
					hdr.Typeflag = hdr.Xattrs["backup.type"][0]
				}
				if s, err := strconv.ParseInt(hdr.Xattrs["backup.size"], 0, 0); err == nil {
					hdr.Size = s
				}

				switch check(*hdr, true) {
				case OK:
					delete(backupset, hdr.Name)
				}

			}
		}

		if err := cmd.Wait(); err != nil {
			fmt.Println("Backend error:", err)
			log.Fatal(cmd.Args, err)
		}

		if verbose {
			fmt.Println("done.")
			fmt.Printf("Incremental backup: date=%d files=%d\n", date, len(backupset))
		}
		log.Printf("Incremental backup: date=%d files=%d\n", date, len(backupset))
	}

	dumpfiles()
	log.Printf("Finished backup: date=%d name=%q schedule=%q files=%d\n", date, name, schedule, len(backupset))
}

func resume() {
	date = 0
	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.Parse()

	log.Printf("Resuming backup: date=%d\n", date)
	if verbose {
		fmt.Printf("Resuming backup: date=%d\n", date)
	}

	backupset = make(map[string]struct{})

	cmd := remotecommand("metadata", "-name", name, "-date", fmt.Sprintf("%d", date))

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
			name = hdr.Name
			schedule = hdr.Linkname
			date = BackupID(hdr.ModTime.Unix())
			hdr.ChangeTime = time.Unix(int64(hdr.Uid), 0)
			if hdr.ChangeTime.Unix() != 0 {
				fmt.Printf("Error: backup set date=%d is already complete\n", date)
				log.Fatalf("Error: backup set date=%d is already complete\n", date)
			}
			if verbose {
				fmt.Print("Determining files to backup... ")
			}
		default:
			if len(hdr.Xattrs["backup.type"]) > 0 {
				hdr.Typeflag = hdr.Xattrs["backup.type"][0]
			}
			if s, err := strconv.ParseInt(hdr.Xattrs["backup.size"], 0, 0); err == nil {
				hdr.Size = s
			}

			if check(*hdr, true) != OK {
				backupset[hdr.Name] = struct{}{}
			}

		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if verbose {
		fmt.Println("done.")
		fmt.Printf("Incremental backup: date=%d files=%d\n", date, len(backupset))
	}
	log.Printf("Incremental backup: date=%d files=%d\n", date, len(backupset))
	dumpfiles()
}

func dumpfiles() {
	if verbose {
		fmt.Print("Sending files... ")
	}

	cmd := remotecommand("submitfiles", "-name", name, "-date", fmt.Sprintf("%d", date))
	stdin, err := cmd.StdinPipe()
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
		ModTime:  time.Unix(int64(date), 0),
		Typeflag: tar.TypeXGlobalHeader,
	}
	tw.WriteHeader(globalhdr)
	tw.Write(globaldata)

	for f := range backupset {
		if fi, err := os.Lstat(f); err != nil {
			log.Println(err)
			if os.IsNotExist(err) {
				hdr := &tar.Header{
					Name:     f,
					Typeflag: 'X',
				}
				tw.WriteHeader(hdr)
			}
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
	date = 0

	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", "", "Backup name")
	flag.StringVar(&name, "n", "", "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.Parse()

	args := []string{"metadata"}
	if date != 0 {
		args = append(args, "-date", fmt.Sprintf("%d", date))
	}
	if name != "" {
		args = append(args, "-name", name)
	}
	args = append(args, flag.Args()...)
	cmd := remotecommand(args...)

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
	first := true
	var size int64 = 0
	var files int64 = 0
	var missing int64 = 0
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
			if !first {
				fmt.Println()
			}
			first = false
			size = 0
			files = 0
			missing = 0

			var header BackupInfo
			dec := gob.NewDecoder(tr)
			if err := dec.Decode(&header); err != nil {
				fmt.Println("Protocol error:", err)
				log.Fatal(err)
			}

			fmt.Println("Date:    ", header.Date)
			fmt.Println("Name:    ", header.Name)
			fmt.Println("Schedule:", header.Schedule)
			fmt.Println("Started: ", time.Unix(int64(header.Date), 0))
			if header.Finished.Unix() != 0 {
				fmt.Println("Finished:", header.Finished)
				fmt.Println("Duration:", header.Finished.Sub(time.Unix(int64(header.Date), 0)))
			}
			if header.Files > 0 {
				fmt.Println("Size:    ", Bytes(uint64(header.Size)))
				fmt.Println("Files:   ", header.Files)
			}
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
					fmt.Printf("%8s", Bytes(s))
				} else {
					fmt.Printf("%8s", "")
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
		fmt.Print("Complete: ")
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

func ping() {
	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.Parse()

	if verbose {
		if len(cfg.Server) > 0 {
			fmt.Println("Server:", cfg.Server)
		}
		if cfg.Port > 0 {
			fmt.Println("Port:", cfg.Port)
		}
		if len(cfg.User) > 0 {
			fmt.Println("User:", cfg.User)
		}
	}

	cmd := remotecommand("version")

	if verbose {
		fmt.Println("Backend:", cmd.Path)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if verbose {
		fmt.Println()
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", cmd.Args, err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", cmd.Args, err)
		fmt.Println("Backend error:", ExitCode(cmd.ProcessState))
		log.Fatal(cmd.Args, err)
	}
}

func register() {
	ClientOnly()

	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.Parse()

	if len(cfg.Server) < 1 {
		fmt.Println("Error registering client: no server configured")
		log.Fatal("Error registering client: no server configured")
	}

	if verbose {
		if len(cfg.Server) > 0 {
			fmt.Println("Server:", cfg.Server)
		}
		if cfg.Port > 0 {
			fmt.Println("Port:", cfg.Port)
		}
		if len(cfg.User) > 0 {
			fmt.Println("User:", cfg.User)
		}
	}

	if err := sshcopyid(); err != nil {
		fmt.Println("Error registering client:", err)
		log.Fatal("Error registering client:", err)
	}

	if verbose {
		fmt.Println("Registered to server:", cfg.Server)
	}
	log.Println("Registered to server:", cfg.Server)
}

func check(hdr tar.Header, quick bool) (result Status) {
	result = Unknown

	if hdr.Typeflag == '?' {
		result = Missing
		return
	}

	if fi, err := os.Lstat(hdr.Name); err == nil {
		fhdr, err := tar.FileInfoHeader(fi, hdr.Linkname)
		if err != nil {
			return
		} else {
			fhdr.Uname = Username(fhdr.Uid)
			fhdr.Gname = Groupname(fhdr.Gid)
		}
		result = OK
		if fhdr.Mode != hdr.Mode ||
			fhdr.Uid != hdr.Uid ||
			fhdr.Gid != hdr.Gid ||
			fhdr.Uname != hdr.Uname ||
			fhdr.Gname != hdr.Gname ||
			!fhdr.ModTime.IsZero() && !hdr.ModTime.IsZero() && fhdr.ModTime.Unix() != hdr.ModTime.Unix() ||
			!fhdr.AccessTime.IsZero() && !hdr.AccessTime.IsZero() && fhdr.AccessTime.Unix() != hdr.AccessTime.Unix() ||
			!fhdr.ChangeTime.IsZero() && !hdr.ChangeTime.IsZero() && fhdr.ChangeTime.Unix() != hdr.ChangeTime.Unix() ||
			fhdr.Typeflag != hdr.Typeflag ||
			fhdr.Typeflag == tar.TypeSymlink && fhdr.Linkname != hdr.Linkname {
			result = MetaModified
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return
		}
		if hdr.Size != fhdr.Size {
			result = Modified
			return
		}

		if quick && result != OK {
			return
		}

		if hdr.Xattrs["backup.hash"] != Hash(hdr.Name) {
			result = Modified
		}
	} else {
		if os.IsNotExist(err) {
			result = Deleted
		}
		return
	}

	return
}

func verify() {
	date = 0

	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.Parse()

	args := []string{"metadata"}
	args = append(args, "-date", fmt.Sprintf("%d", date))
	if name != "" {
		args = append(args, "-name", name)
	}
	args = append(args, flag.Args()...)
	cmd := remotecommand(args...)

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
	first := true
	var size int64 = 0
	var files int64 = 0
	var missing int64 = 0
	var modified int64 = 0
	var deleted int64 = 0
	var errors int64 = 0
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
			if !first {
				fmt.Println()
			}
			first = false
			size = 0
			files = 0
			missing = 0
			modified = 0
			deleted = 0
			fmt.Println("Name:    ", hdr.Name)
			fmt.Println("Schedule:", hdr.Linkname)
			fmt.Println("Date:    ", hdr.ModTime.Unix(), "(", hdr.ModTime, ")")
		default:
			status := "?"
			files++
			if len(hdr.Xattrs["backup.type"]) > 0 {
				hdr.Typeflag = hdr.Xattrs["backup.type"][0]
			}
			if s, err := strconv.ParseInt(hdr.Xattrs["backup.size"], 0, 0); err == nil {
				hdr.Size = s
			}

			size += hdr.Size

			switch check(*hdr, false) {
			case OK:
				status = ""
			case Modified:
				status = "M"
				modified++
			case Missing:
				status = "+"
				missing++
			case MetaModified:
				status = "m"
				modified++
			case Deleted:
				status = "-"
				deleted++
			case Unknown:
				status = "!"
				errors++
			}

			if verbose && status != "" {
				fmt.Printf("%s %s\n", status, hdr.Name)
			}
		}
	}
	if files > 0 {
		fmt.Println("Size:    ", Bytes(uint64(size)))
		fmt.Println("Files:   ", files)
		fmt.Println("Modified:", modified)
		fmt.Println("Deleted: ", deleted)
		fmt.Println("Missing: ", missing)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
}

func purge() {
	date = 0

	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.Parse()

	args := []string{"purgebackup"}
	if date != 0 {
		args = append(args, "-date", fmt.Sprintf("%d", date))
	}
	if name != "" {
		args = append(args, "-name", name)
	}
	args = append(args, flag.Args()...)
	cmd := remotecommand(args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
}

func archive() {
	gz := false
	var output string
	date = BackupID(time.Now().Unix())

	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.StringVar(&output, "file", "", "Output file")
	flag.StringVar(&output, "o", "", "-file")
	flag.StringVar(&output, "f", "", "-file")
	flag.BoolVar(&gz, "gzip", gz, "Compress archive using gzip")
	flag.BoolVar(&gz, "z", gz, "-gzip")
	flag.Parse()

	if output == "" {
		fmt.Println("Missing output file")
		os.Exit(1)
	}

	args := []string{"data"}
	args = append(args, "-date", fmt.Sprintf("%d", date))
	args = append(args, "-name", name)
	args = append(args, flag.Args()...)
	cmd := remotecommand(args...)

	if output == "-" {
		cmd.Stdout = os.Stdout
	} else {
		out, err := os.Create(output)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			log.Fatal(err)
		}

		cmd.Stdout = out
	}
	cmd.Stderr = os.Stderr

	if gz {
		gzw := gzip.NewWriter(cmd.Stdout)
		defer gzw.Close()
		cmd.Stdout = gzw
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
}

func expire() {
	keep := 0
	if IsServer() {
		name = ""
	} else {
		name = defaultName
	}
	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", name, "Backup name")
	flag.StringVar(&name, "n", name, "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
	flag.IntVar(&keep, "keep", keep, "Minimum number of backups to keep")
	flag.IntVar(&keep, "k", keep, "-keep")

	flag.Var(&date, "age", "Maximum age/date")
	flag.Var(&date, "a", "-age")
	flag.Var(&date, "date", "-age")
	flag.Var(&date, "d", "-age")
	flag.Parse()

	if verbose {
		fmt.Printf("Expiring backups for %q, schedule %q\n", name, schedule)
	}

	args := []string{"expirebackup"}
	if date > 0 {
		args = append(args, "-date", fmt.Sprintf("%d", date))
	}
	if name != "" {
		args = append(args, "-name", name)
	}
	if keep > 0 {
		args = append(args, "-keep", fmt.Sprintf("%d", keep))
	}
	args = append(args, "-schedule", schedule)
	cmd := remotecommand(args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
}

func restore() {
	date = BackupID(time.Now().Unix())

	flag.BoolVar(&verbose, "verbose", verbose, "Be more verbose")
	flag.BoolVar(&verbose, "v", verbose, "-verbose")
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.Parse()

	args := []string{"data"}
	args = append(args, "-date", fmt.Sprintf("%d", date))
	args = append(args, "-name", name)
	args = append(args, flag.Args()...)
	getdata := remotecommand(args...)
	getdata.Stderr = os.Stderr

	args = []string{}
	args = append(args, "-x", "-p", "-f", "-")
	if verbose {
		args = append(args, "-v")
	}
	tar := exec.Command("tar", args...)
	tar.Stderr = os.Stderr
	tar.Stdout = os.Stdout

	tar.Stdin, _ = getdata.StdoutPipe()

	if err := tar.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(tar.Args, err)
	}
	if err := getdata.Run(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(getdata.Args, err)
	}
	if err := tar.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(tar.Args, err)
	}
}
