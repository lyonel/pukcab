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
	"strings"
	"time"
)

func backup() {
	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.StringVar(&schedule, "schedule", "", "Backup schedule")
	flag.StringVar(&schedule, "r", "", "-schedule")
	flag.BoolVar(&full, "full", full, "Full backup")
	flag.BoolVar(&full, "f", full, "-full")
	Setup()

	if len(flag.Args()) != 0 {
		failure.Fatal("Too many parameters: ", strings.Join(flag.Args(), " "))
	}

	if err := dobackup(name, schedule, full); err != nil {
		failure.Fatal("Backup failure.")
	}
}

func dobackup(name string, schedule string, full bool) (fail error) {
	info.Printf("Starting backup: name=%q schedule=%q\n", name, schedule)
	log.Printf("Starting backup: name=%q schedule=%q\n", name, schedule)

	backup := NewBackup(cfg)

	info.Print("Sending file list... ")

	cmdline := []string{"newbackup", "-name", name, "-full=" + strconv.FormatBool(full)}
	if schedule != "" {
		cmdline = append(cmdline, "-schedule", schedule)
	}
	if force {
		cmdline = append(cmdline, "-force")
	}

	backup.Start(name, schedule)

	cmd := remotecommand(cmdline...)
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
		info.Printf("New backup: date=%d name=%q files=%d\n", backup.Date, backup.Name, backup.Count())
		log.Printf("New backup: date=%d name=%q files=%d\n", backup.Date, backup.Name, backup.Count())
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
		info.Print("Determining files to backup... ")
		if err := checkmetadata(backup); err != nil {
			return err
		}
		backuptype := "incremental"
		if files == backup.Count() {
			backuptype = "full"
		}
		info.Println("done.")
		info.Printf("Backup: date=%d name=%q files=%d type=%s\n", backup.Date, backup.Name, backup.Count(), backuptype)
		log.Printf("Backup: date=%d name=%q files=%d type=%s\n", backup.Date, backup.Name, backup.Count(), backuptype)
	}

	bytes := dumpfiles(files, backup)
	log.Printf("Finished sending: date=%d name=%q schedule=%q files=%d sent=%d duration=%.0f\n", backup.Date, name, schedule, backup.Count(), bytes, time.Since(backup.Started).Seconds())

	return
}

func process(c string, backup *Backup, action func(tar.Header), files ...string) (fail error) {
	args := []string{c}
	if backup.Date != 0 {
		args = append(args, "-date", fmt.Sprintf("%d", backup.Date))
	}
	if backup.Name != "" {
		args = append(args, "-name", backup.Name)
	}
	if backup.Schedule != "" {
		args = append(args, "-schedule", backup.Schedule)
	}
	args = append(args, files...)
	cmd := remotecommand(args...)

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
			var header BackupInfo
			dec := gob.NewDecoder(tr)
			if err := dec.Decode(&header); err != nil {
				failure.Println("Protocol error:", err)
				log.Println(err)
				return err
			}
			backup.Name, backup.Schedule, backup.Date, backup.Finished, backup.LastModified =
				header.Name, header.Schedule, header.Date, header.Finished, header.LastModified
			hdr.Name, hdr.Linkname, hdr.ModTime, hdr.ChangeTime, hdr.AccessTime, hdr.Size =
				header.Name, header.Schedule, time.Unix(int64(header.Date), 0), header.Finished, header.LastModified, header.Size
			if hdr.Xattrs == nil {
				hdr.Xattrs = make(map[string]string)
			}
			hdr.Xattrs["backup.files"] = fmt.Sprintf("%d", header.Files)
			hdr.Xattrs["backup.schedule"] = header.Schedule

			action(*hdr)
		default:
			if len(hdr.Xattrs["backup.type"]) > 0 {
				hdr.Typeflag = hdr.Xattrs["backup.type"][0]
			}
			if s, err := strconv.ParseInt(hdr.Xattrs["backup.size"], 0, 0); err == nil {
				hdr.Size = s
			}

			action(*hdr)
		}
	}

	if err := cmd.Wait(); err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return err
	}

	return
}

func checkmetadata(backup *Backup, files ...string) (fail error) {
	return process("metadata", backup, func(hdr tar.Header) {
		if Check(hdr, true) == OK {
			backup.Forget(hdr.Name)
		}
	}, files...)
}

func resume() {
	var date BackupID = 0

	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	Setup()

	if len(flag.Args()) != 0 {
		failure.Fatal("Too many parameters: ", strings.Join(flag.Args(), " "))
	}

	if err := doresume(date, name); err != nil {
		failure.Fatal("Backup failure.")
	}
}

func doresume(date BackupID, name string) (fail error) {
	log.Printf("Resuming backup: date=%d\n", date)
	info.Printf("Resuming backup: date=%d\n", date)

	backup := NewBackup(cfg)
	backup.Init(date, name)
	if err := checkmetadata(backup); err != nil {
		return err
	}

	if !backup.Finished.IsZero() && backup.Finished.Unix() != 0 {
		failure.Printf("Backup set date=%d is already complete\n", backup.Date)
		log.Printf("Backup set date=%d is already complete\n", backup.Date)
		return nil
	}

	info.Print("Determining files to backup... ")
	if date == 0 { // force retrieving the file list
		if err := checkmetadata(backup); err != nil {
			return err
		}
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
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

	if err := cmd.Start(); err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return
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
		debug.Println("Sending", f)
		if fi, err := os.Lstat(f); err != nil {
			if os.IsNotExist(err) {
				hdr := &tar.Header{
					Name:     f,
					Typeflag: 'X',
				}
				tw.WriteHeader(hdr)
			} else {
				log.Println(err)
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
								info.Println("Could not read ", f, ": ", er)
								log.Println("Could not read ", f, ": ", er)
							}
							if nr > 0 {
								nw, ew := tw.Write(buf[0:nr])
								if ew != nil {
									if ew == tar.ErrWriteTooLong {
										break
									}
									failure.Println("Could not send ", f, ": ", ew)
									log.Println("Could not send ", f, ": ", ew)
									return
								} else {
									written += int64(nw)
								}
							}
						}
						file.Close()

						if written < hdr.Size { // short write: fill with zeros
							tw.Write(make([]byte, hdr.Size-written))
						}

						if written != hdr.Size {
							log.Printf("Could not backup file=%q msg=\"size changed during backup\" name=%q date=%d error=warn\n", f, name, backup.Date)
						}
						bytes += written
					}
				} else {
					tw.WriteHeader(hdr)
				}
				done++
			} else {
				log.Printf("Couldn't backup file=%q msg=%q name=%q date=%d error=warn\n", f, err, name, backup.Date)
			}
		}
	})

	stdin.Close()

	if err := cmd.Wait(); err != nil {
		failure.Println("Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

	info.Println("done.")
	info.Println(Bytes(uint64(float32(bytes)/float32(time.Since(backup.Started).Seconds()))) + "/s")

	return bytes
}

func history() {
	date = 0
	name = ""
	schedule = ""
	flag.StringVar(&name, "name", name, "Backup name")
	flag.StringVar(&name, "n", name, "-name")
	flag.StringVar(&schedule, "schedule", schedule, "Backup schedule")
	flag.StringVar(&schedule, "r", schedule, "-schedule")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	Setup()

	if name == "" && !cfg.IsServer() {
		name = defaultName
	}

	if name == "*" {
		name = ""
	}

	backup := NewBackup(cfg)
	backup.Init(date, name)
	backup.Schedule = schedule

	first := true
	if err := process("timeline", backup, func(hdr tar.Header) {
		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:

			if !first {
				fmt.Println()
			}
			first = false

			fmt.Print(hdr.ModTime.Unix(), " ", hdr.Name, " ", hdr.Xattrs["backup.schedule"])
			if !hdr.ChangeTime.IsZero() && hdr.ChangeTime.Unix() != 0 {
				fmt.Print(" ", hdr.ChangeTime.Format("Mon Jan 2 15:04"))
			}
			fmt.Println()
		default:
			if hdr.Typeflag != '?' {
				fmt.Printf("%s %8s %-8s", hdr.FileInfo().Mode(), hdr.Uname, hdr.Gname)
				if s, err := strconv.ParseUint(hdr.Xattrs["backup.size"], 0, 0); err == nil {
					fmt.Printf("%8s", Bytes(s))
				} else {
					fmt.Printf("%8s", "")
				}
				fmt.Print(" ", DisplayTime(hdr.ModTime))
				fmt.Printf(" %s", hdr.Name)
				if hdr.Linkname != "." {
					fmt.Printf(" ➙ %s\n", hdr.Linkname)
				} else {
					fmt.Println()
				}
			}
		}
	}, flag.Args()...); err != nil {
		failure.Println(err)
		log.Fatal(err)
	}
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

	if date == 0 && len(flag.Args()) != 0 {
		date.Set("now")
	}

	backup := NewBackup(cfg)
	backup.Init(date, name)

	if date != 0 {
		verbose = true
	}

	first := true
	var size int64 = 0
	var files int64 = 0
	var missing int64 = 0
	if err := process("metadata", backup, func(hdr tar.Header) {
		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			if !short && !first {
				fmt.Println()
			}
			first = false
			size = 0
			files = 0
			missing = 0

			if !short {
				fmt.Println("Date:    ", hdr.ModTime.Unix())
				fmt.Println("Name:    ", hdr.Name)
				fmt.Println("Schedule:", hdr.Xattrs["backup.schedule"])
				fmt.Println("Started: ", hdr.ModTime)
				if !hdr.ChangeTime.IsZero() && hdr.ChangeTime.Unix() != 0 {
					fmt.Println("Finished:", hdr.ChangeTime)
					fmt.Println("Duration:", hdr.ChangeTime.Sub(hdr.ModTime))
				}
				if !hdr.AccessTime.IsZero() && hdr.AccessTime.Unix() != 0 {
					fmt.Println("Active:  ", time.Now().Sub(hdr.AccessTime))
				}
				if hdr.Size > 0 {
					fmt.Println("Size:    ", Bytes(uint64(hdr.Size)))
					fmt.Println("Files:   ", hdr.Xattrs["backup.files"])
				}
			} else {
				fmt.Print(hdr.ModTime.Unix(), " ", hdr.Name, " ", hdr.Xattrs["backup.schedule"])
				if !hdr.ChangeTime.IsZero() && hdr.ChangeTime.Unix() != 0 {
					fmt.Print(" ", hdr.ChangeTime.Format("Mon Jan 2 15:04"))
				}
				fmt.Println()
			}
		default:
			files++
			size += hdr.Size
			if hdr.Typeflag == '?' {
				missing++
			}
			if verbose {
				fmt.Printf("%s %8s %-8s", hdr.FileInfo().Mode(), hdr.Uname, hdr.Gname)
				if s, err := strconv.ParseUint(hdr.Xattrs["backup.size"], 0, 0); err == nil {
					fmt.Printf("%8s", Bytes(s))
				} else {
					fmt.Printf("%8s", "")
				}
				fmt.Print(" ", DisplayTime(hdr.ModTime))
				fmt.Printf(" %s", hdr.Name)
				if hdr.Linkname != "." {
					fmt.Printf(" ➙ %s\n", hdr.Linkname)
				} else {
					fmt.Println()
				}
			}
		}
	}, flag.Args()...); err != nil {
		failure.Println(err)
		log.Fatal(err)
	}
	if files > 0 {
		fmt.Print("Complete: ")
		if files > 0 && missing > 0 {
			fmt.Printf("%.1f%% (%d files missing)\n", 100*float32(files-missing)/float32(files), missing)
		} else {
			fmt.Println("yes")
		}
	}
}

type Client struct {
	First time.Time
	Last  map[string]time.Time
	Size  int64
	Count int64
}

func dashboard() {
	date = 0
	name = ""
	flag.StringVar(&name, "name", name, "Backup name")
	flag.StringVar(&name, "n", name, "-name")
	Setup()

	if len(flag.Args()) != 0 {
		failure.Fatal("Too many parameters: ", strings.Join(flag.Args(), " "))
	}

	if name == "" && !cfg.IsServer() {
		name = defaultName
	}

	if name == "*" {
		name = ""
	}

	if date == 0 && len(flag.Args()) != 0 {
		date.Set("now")
	}

	backup := NewBackup(cfg)
	backup.Init(date, name)

	clients := make(map[string]Client)
	schedules := make(map[string]struct{})
	if err := process("metadata", backup, func(hdr tar.Header) {
		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			if client, exists := clients[hdr.Name]; exists {
				if hdr.ModTime.Before(client.First) {
					client.First = hdr.ModTime
				}
				if hdr.ModTime.After(client.Last[hdr.Xattrs["backup.schedule"]]) {
					client.Last[hdr.Xattrs["backup.schedule"]] = hdr.ModTime
				}
				if hdr.Size > client.Size {
					client.Size = hdr.Size
				}
				client.Count++

				clients[hdr.Name] = client
			} else {
				client = Client{Last: make(map[string]time.Time)}
				client.First = hdr.ModTime
				client.Last[hdr.Xattrs["backup.schedule"]] = hdr.ModTime
				client.Size = hdr.Size
				client.Count++

				clients[hdr.Name] = client
			}
			schedules[hdr.Xattrs["backup.schedule"]] = struct{}{}
		}
	}, flag.Args()...); err != nil {
		failure.Println(err)
		log.Fatal(err)
	}

	delete(schedules, "daily")
	delete(schedules, "weekly")
	delete(schedules, "monthly")
	delete(schedules, "yearly")
	allschedules := []string{"daily", "weekly", "monthly", "yearly"}
	for s, _ := range schedules {
		allschedules = append(allschedules, s)
	}

	for name, client := range clients {
		fmt.Println("Name:          ", name)
		fmt.Println("Backups:       ", client.Count)
		fmt.Println("First:         ", client.First)
		for _, schedule := range allschedules {
			if when, ok := client.Last[schedule]; ok {
				fmt.Printf("Last %-10s %v\n", schedule+":", when)
			}
		}
		fmt.Println()
	}
}

func ping() {
	Setup()

	if len(flag.Args()) != 0 {
		failure.Fatal("Too many parameters: ", strings.Join(flag.Args(), " "))
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

	cmd := remotecommand("version")

	info.Println("Backend:", cmd.Path)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	info.Println()

	if err := cmd.Start(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Backend error:", err)
		log.Fatal(cmd.Args, err)
	}
}

func register() {
	Setup()
	cfg.ClientOnly()

	if len(flag.Args()) != 0 {
		failure.Fatal("Too many parameters: ", strings.Join(flag.Args(), " "))
	}

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

	if date == 0 {
		date.Set("now")
	}

	Setup()

	backup := NewBackup(cfg)
	backup.Init(date, name)

	first := true
	var size int64 = 0
	var files int64 = 0
	var missing int64 = 0
	var modified int64 = 0
	var deleted int64 = 0
	var errors int64 = 0
	if err := process("metadata", backup, func(hdr tar.Header) {
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

			switch Check(hdr, false) {
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
	}, flag.Args()...); err != nil {
		failure.Println(err)
		log.Fatal(err)
	}
	if files > 0 {
		fmt.Println("Size:    ", Bytes(uint64(size)))
		fmt.Println("Files:   ", files)
		fmt.Println("Modified:", modified)
		fmt.Println("Deleted: ", deleted)
		fmt.Println("Missing: ", missing)
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

	if len(flag.Args()) != 0 {
		failure.Fatal("Too many parameters: ", strings.Join(flag.Args(), " "))
	}

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
	date.Set("now")

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

	if len(flag.Args()) != 0 {
		failure.Fatal("Too many parameters: ", strings.Join(flag.Args(), " "))
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
	date.Set("now")

	directory := ""
	inplace := false

	flag.StringVar(&name, "name", defaultName, "Backup name")
	flag.StringVar(&name, "n", defaultName, "-name")
	flag.StringVar(&directory, "directory", "", "Change to directory")
	flag.StringVar(&directory, "C", "", "-directory")
	flag.Var(&date, "date", "Backup set")
	flag.Var(&date, "d", "-date")
	flag.BoolVar(&inplace, "in-place", inplace, "Restore in-place")
	flag.BoolVar(&inplace, "inplace", inplace, "-in-place")

	Setup()

	if inplace {
		if directory != "" && directory != "/" {
			failure.Fatal("Inconsistent parameters")
		} else {
			directory = "/"
		}
	}

	args := []string{"data"}
	args = append(args, "-date", fmt.Sprintf("%d", date))
	args = append(args, "-name", name)
	args = append(args, flag.Args()...)
	getdata := remotecommand(args...)
	getdata.Stderr = os.Stderr

	args = []string{}
	args = append(args, "-x", "-p", "-f", "-")
	if directory != "" {
		args = append(args, "-C", directory)
	}
	if verbose {
		args = append(args, "-v")
	}
	tar := exec.Command(cfg.Tar, args...)
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
