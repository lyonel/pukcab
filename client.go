package main

import (
	"bufio"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"ezix.org/tar"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func backup() {
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.StringVar(&schedule, "schedule", defaultSchedule, "Backup schedule")
	flag.StringVar(&schedule, "r", defaultSchedule, "-schedule")
	flag.BoolVar(&full, "full", full, "Full backup")
	flag.BoolVar(&full, "f", full, "-full")
	Setup()

	if err := dobackup(name, schedule, full); err != nil {
		failure.Fatal("Backup failure.")
	}
}

func dobackup(name string, schedule string, full bool) (fail error) {
	info.Printf("Starting backup: name=%q schedule=%q\n", name, schedule)

	backup := NewBackup(cfg)
	backup.Start(name, schedule)

	info.Print("Sending file list... ")

	cmd := remotecommand("newbackup", "-name", name, "-schedule", schedule, "-full="+strconv.FormatBool(full))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}

	if err := cmd.Start(); err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}

	if protocol > 0 {
		backup.ForEach(func(f string) { fmt.Fprintln(stdin, strconv.Quote(f)) })
	} else {
		backup.ForEach(func(f string) { fmt.Fprintln(stdin, f) })
	}
	stdin.Close()

	info.Println("done.")

	var previous int64 = 0
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		if d, err := strconv.ParseInt(scanner.Text(), 10, 0); err != nil {
			failure.Println("Protocol error")
			log.Println("Protocol error")
			return err
		} else {
			backup.Date = BackupID(d)
		}
	}

	if backup.Date == 0 {
		scanner.Scan()
		errmsg := scanner.Text()
		failure.Println("Server error", errmsg)
		log.Println("Server error", errmsg)
		return errors.New("Server error")
	} else {
		info.Printf("New backup: date=%d files=%d\n", backup.Date, backup.Count())
		log.Printf("New backup: date=%d files=%d\n", backup.Date, backup.Count())
		if scanner.Scan() {
			previous, _ = strconv.ParseInt(scanner.Text(), 10, 0)
			if previous > 0 {
				info.Printf("Previous backup: date=%d\n", previous)
				log.Printf("Previous backup: date=%d\n", previous)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}

	files := backup.Count()

	if !full {
		cmd = remotecommand("metadata", "-name", name, "-date", fmt.Sprintf("%d", backup.Date))

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			failure.Println("Backend error:", err)
			log.Println(cmd.Args, err)
			return err
		}

		if err := cmd.Start(); err != nil {
			failure.Println("Backend error:", err)
			log.Println(cmd.Args, err)
			return err
		}

		tr := tar.NewReader(stdout)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				failure.Println("Backend error:", err)
				log.Println(err)
				return err
			}

			switch hdr.Typeflag {
			case tar.TypeXGlobalHeader:
				info.Print("Determining files to backup... ")
			default:
				if len(hdr.Xattrs["backup.type"]) > 0 {
					hdr.Typeflag = hdr.Xattrs["backup.type"][0]
				}
				if s, err := strconv.ParseInt(hdr.Xattrs["backup.size"], 0, 0); err == nil {
					hdr.Size = s
				}

				switch Check(*hdr, true) {
				case OK:
					backup.Forget(hdr.Name)
				}

			}
		}

		if err := cmd.Wait(); err != nil {
			failure.Println("Backend error:", err)
			log.Println(cmd.Args, err)
			return err
		}

		backuptype := "incremental"
		if files == backup.Count() {
			backuptype = "full"
		}
		info.Println("done.")
		info.Printf("Backup: date=%d files=%d type=%q\n", backup.Date, backup.Count(), backuptype)
		log.Printf("Backup: date=%d files=%d type=%q\n", backup.Date, backup.Count(), backuptype)
	}

	bytes := dumpfiles(files, backup)
	log.Printf("Finished sending: date=%d name=%q schedule=%q files=%d sent=%d duration=%.0f\n", backup.Date, name, schedule, backup.Count(), bytes, time.Since(backup.Started).Seconds())

	return
}

func resume() {
	var date BackupID = 0

	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	Setup()

	if err := doresume(date, name); err != nil {
		failure.Fatal("Backup failure.")
	}
}

func doresume(date BackupID, name string) (fail error) {
	log.Printf("Resuming backup: date=%d\n", date)
	info.Printf("Resuming backup: date=%d\n", date)

	backup := NewBackup(cfg)

	cmd := remotecommand("metadata", "-name", name, "-date", fmt.Sprintf("%d", date))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}

	if err := cmd.Start(); err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}

	tr := tar.NewReader(stdout)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			failure.Println("Backend error:", err)
			log.Println(err)
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			backup.Name = hdr.Name
			backup.Schedule = hdr.Linkname
			backup.Date = BackupID(hdr.ModTime.Unix())
			hdr.ChangeTime = time.Unix(int64(hdr.Uid), 0)
			if hdr.ChangeTime.Unix() != 0 {
				failure.Printf("Error: backup set date=%d is already complete\n", backup.Date)
				log.Printf("Error: backup set date=%d is already complete\n", backup.Date)
				return nil
			}
			info.Print("Determining files to backup... ")
		default:
			if len(hdr.Xattrs["backup.type"]) > 0 {
				hdr.Typeflag = hdr.Xattrs["backup.type"][0]
			}
			if s, err := strconv.ParseInt(hdr.Xattrs["backup.size"], 0, 0); err == nil {
				hdr.Size = s
			}

			if Check(*hdr, true) != OK {
				backup.Add(hdr.Name)
			}

		}
	}

	if err := cmd.Wait(); err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}

	info.Println("done.")
	info.Printf("Resuming backup: date=%d files=%d\n", backup.Date, backup.Count())
	log.Printf("Resuming backup: date=%d files=%d\n", backup.Date, backup.Count())
	dumpfiles(backup.Count(), backup)

	return
}

func dumpfiles(files int, backup *Backup) (bytes int64) {
	done := files - backup.Count()
	bytes = 0

	info.Print("Sending files... ")

	cmd := remotecommand("submitfiles", "-name", backup.Name, "-date", fmt.Sprintf("%d", backup.Date))
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
		".name":     backup.Name,
		".schedule": backup.Schedule,
		".version":  fmt.Sprintf("%d.%d", versionMajor, versionMinor),
	})
	globalhdr := &tar.Header{
		Name:     backup.Name,
		Size:     int64(len(globaldata)),
		Linkname: backup.Schedule,
		ModTime:  time.Unix(int64(backup.Date), 0),
		Typeflag: tar.TypeXGlobalHeader,
	}
	tw.WriteHeader(globalhdr)
	tw.Write(globaldata)

	backup.ForEach(func(f string) {
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
						bytes += written
					}
				} else {
					tw.WriteHeader(hdr)
				}
				done++
			} else {
				log.Printf("Couldn't backup %s: %s\n", f, err)
			}
		}
	})

	stdin.Close()

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	info.Println("done.")
	info.Println(Bytes(uint64(float64(bytes)/time.Since(backup.Started).Seconds())) + "/s")

	return bytes
}

func list() {
	date = 0
	name = ""
	short := false
	flag.StringVar(&name, "name", name, "Backup name")
	flag.StringVar(&name, "n", name, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.BoolVar(&short, "short", short, "Concise output")
	flag.BoolVar(&short, "s", short, "-short")
	Setup()

	if name == "" && !cfg.IsServer() {
		name = defaultName
	}

	if name == "*" {
		name = ""
	}

	args := []string{"metadata"}
	if date != 0 {
		args = append(args, "-date", fmt.Sprintf("%d", date))
		verbose = true
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
			if !short && !first {
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

			if !short {
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
			} else {
				fmt.Print(header.Date, " ", header.Name, " ", header.Schedule)
				if header.Finished.Unix() != 0 {
					fmt.Print(" ", header.Finished.Format("Mon Jan 2 15:04"))
				}
				fmt.Println()
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
	Setup()

	if len(cfg.Server) > 0 {
		info.Println("Server:", cfg.Server)
	}
	if cfg.Port > 0 {
		info.Println("Port:", cfg.Port)
	}
	if len(cfg.User) > 0 {
		info.Println("User:", cfg.User)
	}

	cmd := remotecommand("version")

	info.Println("Backend:", cmd.Path)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	info.Println()

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
	Setup()
	cfg.ClientOnly()

	if len(cfg.Server) < 1 {
		fmt.Println("Error registering client: no server configured")
		log.Fatal("Error registering client: no server configured")
	}

	if len(cfg.Server) > 0 {
		info.Println("Server:", cfg.Server)
	}
	if cfg.Port > 0 {
		info.Println("Port:", cfg.Port)
	}
	if len(cfg.User) > 0 {
		info.Println("User:", cfg.User)
	}

	if err := sshcopyid(); err != nil {
		fmt.Println("Error registering client:", err)
		log.Fatal("Error registering client:", err)
	}

	info.Println("Registered to server:", cfg.Server)
	log.Println("Registered to server:", cfg.Server)
}

func verify() {
	date = 0

	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	Setup()

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

			switch Check(*hdr, false) {
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
	name = ""

	flag.StringVar(&name, "name", name, "Backup name")
	flag.StringVar(&name, "n", name, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	Setup()

	if name == "" && !cfg.IsServer() {
		name = defaultName
	}

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

	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.StringVar(&output, "file", "", "Output file")
	flag.StringVar(&output, "o", "", "-file")
	flag.StringVar(&output, "f", "", "-file")
	flag.BoolVar(&gz, "gzip", gz, "Compress archive using gzip")
	flag.BoolVar(&gz, "z", gz, "-gzip")

	Setup()

	if output == "" {
		fmt.Println("Missing output file")
		os.Exit(1)
	}

	if ext := filepath.Ext(output); ext == ".gz" || ext == ".tgz" {
		gz = true
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
	name = ""
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

	Setup()

	if name == "" && !cfg.IsServer() {
		name = defaultName
	}

	info.Printf("Expiring backups for %q, schedule %q\n", name, schedule)

	if name == "*" {
		name = ""
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

	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")

	Setup()

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
