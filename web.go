package main

import (
	"encoding/gob"
	"ezix.org/tar"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Report struct {
	Title string
}

type AboutReport struct {
	Report
	Name          string
	Major         int
	Minor         int
	OS            string
	Arch          string
	CPUs          int
	Goroutines    int
	Bytes, Memory int64
	Load          float64
}

type ConfigReport struct {
	Report
	Config
}

type BackupsReport struct {
	Report
	Names, Schedules []string
	Files, Size      int64
	Backups          []BackupInfo
}

type StorageReport struct {
	Report
	VaultFS, CatalogFS                         string
	VaultCapacity, VaultBytes, VaultFree       int64
	CatalogCapacity, CatalogBytes, CatalogFree int64
	VaultUsed, CatalogUsed                     float32
}

func stylesheets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=UTF-8")
	fmt.Fprintf(w, css)
}

func webhome(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	report := &AboutReport{
		Report: Report{
			Title: programName + " on " + defaultName,
		},
		Name:       defaultName,
		Major:      versionMajor,
		Minor:      versionMinor,
		OS:         strings.ToTitle(runtime.GOOS[:1]) + runtime.GOOS[1:],
		Arch:       runtime.GOARCH,
		CPUs:       runtime.NumCPU(),
		Goroutines: runtime.NumGoroutine(),
		Bytes:      int64(mem.Alloc),
		Memory:     int64(mem.Sys),
		Load:       LoadAvg(),
	}
	pages.ExecuteTemplate(w, "HOME", report)
}

func webconfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	report := &ConfigReport{
		Report: Report{
			Title: programName + " on " + defaultName,
		},
		Config: cfg,
	}
	pages.ExecuteTemplate(w, "CONFIG", report)
}

func webinfo(w http.ResponseWriter, r *http.Request) {
	date = 0
	name = ""

	req := strings.SplitN(r.RequestURI[1:], "/", 3)
	if len(req) > 1 && len(req[1]) > 0 {
		name = req[1]
	}
	if len(req) > 2 && len(req[2]) > 0 {
		if d, err := strconv.Atoi(req[2]); err == nil {
			date = BackupID(d)
		} else {
			http.Error(w, "Invalid request", http.StatusNotAcceptable)
			return
		}
	}

	if name == "" && !cfg.IsServer() {
		name = defaultName
	}

	if name == "*" {
		name = ""
	}

	args := []string{"metadata"}
	if name != "" {
		args = append(args, "-name", name)
	}
	cmd := remotecommand(args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(cmd.Args, err)
		http.Error(w, "Backend error: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Println(cmd.Args, err)
		http.Error(w, "Backend error: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	report := &BackupsReport{
		Report: Report{
			Title: "Backups",
		},
		Names:     []string{},
		Schedules: []string{},
		Backups:   []BackupInfo{},
	}

	names := make(map[string]struct{})
	schedules := make(map[string]struct{})
	tr := tar.NewReader(stdout)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(err)
			http.Error(w, "Backend error: "+err.Error(), http.StatusBadGateway)
			return
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			var header BackupInfo
			dec := gob.NewDecoder(tr)
			if err := dec.Decode(&header); err != nil {
				http.Error(w, "Protocol error: "+err.Error(), http.StatusBadGateway)
				log.Println(err)
				return
			} else {
				if date == 0 || header.Date == date {
					report.Backups = append(report.Backups, header)
					report.Size += header.Size
					report.Files += header.Files
					names[header.Name] = struct{}{}
					schedules[header.Schedule] = struct{}{}
				}
			}
		}
	}

	for n := range names {
		report.Names = append(report.Names, n)
	}
	for s := range schedules {
		report.Schedules = append(report.Schedules, s)
	}

	if err := cmd.Wait(); err != nil {

		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				switch status.ExitStatus() {
				case 2: // retry
					w.Header().Set("Refresh", "10")
					w.Header().Set("Content-Type", "text/html; charset=UTF-8")
					w.WriteHeader(http.StatusAccepted)
					pages.ExecuteTemplate(w, "BUSY", report)
				default:
					log.Println(cmd.Args, err)
					http.Error(w, "Backend error: "+err.Error(), http.StatusServiceUnavailable)
				}
			}
		} else {
			log.Println(cmd.Args, err)
		}

		return
	}

	w.Header().Set("Refresh", "900")
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	if len(report.Backups) == 1 {
		report.Title = "Backup"
		if err := pages.ExecuteTemplate(w, "BACKUP", report); err != nil {
			log.Println(err)
			http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		}
	} else {
		for i, j := 0, len(report.Backups)-1; i < j; i, j = i+1, j-1 {
			report.Backups[i], report.Backups[j] = report.Backups[j], report.Backups[i]
		}
		if err := pages.ExecuteTemplate(w, "BACKUPS", report); err != nil {
			log.Println(err)
			http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

func webdelete(w http.ResponseWriter, r *http.Request) {
	date = 0
	name = ""

	req := strings.SplitN(r.RequestURI[1:], "/", 3)

	if len(req) != 3 {
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
		return
	}

	if len(req[1]) > 0 {
		name = req[1]
	}
	if len(req[2]) > 0 {
		if d, err := strconv.Atoi(req[2]); err == nil {
			date = BackupID(d)
		} else {
			http.Error(w, "Invalid request", http.StatusNotAcceptable)
			return
		}
	}

	if date == 0 || name == "" {
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
		return
	}

	args := []string{"purgebackup", "-name", name, "-date", fmt.Sprintf("%d", date)}
	cmd := remotecommand(args...)

	go cmd.Run()

	http.Redirect(w, r, "/backups/", http.StatusFound)
}

func webnew(w http.ResponseWriter, r *http.Request) {
	report := &ConfigReport{
		Report: Report{
			Title: "New backup",
		},
		Config: cfg,
	}
	if err := pages.ExecuteTemplate(w, "NEW", report); err != nil {
		log.Println(err)
		http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
	}
}

func webstart(w http.ResponseWriter, r *http.Request) {
	setDefaults()

	go dobackup(defaultName, defaultSchedule, false)

	http.Redirect(w, r, "/backups/", http.StatusFound)
}

func webtools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	catalog := cfg.Catalog
	vault := cfg.Vault

	if pw, err := Getpwnam(cfg.User); err == nil {
		if !filepath.IsAbs(cfg.Catalog) {
			catalog = filepath.Join(pw.Dir, cfg.Catalog)
		}
		if !filepath.IsAbs(cfg.Vault) {
			vault = filepath.Join(pw.Dir, cfg.Vault)
		}
	}

	var cstat, vstat syscall.Statfs_t
	if err := syscall.Statfs(catalog, &cstat); err != nil {
		log.Println(err)
	}
	if err := syscall.Statfs(vault, &vstat); err != nil {
		log.Println(err)
	}
	if cstat.Fsid == vstat.Fsid {
		cstat.Bsize = 0
	}

	report := &StorageReport{
		Report: Report{
			Title: "Tools",
		},
		VaultCapacity:   int64(vstat.Bsize) * int64(vstat.Blocks),
		VaultBytes:      int64(vstat.Bsize) * int64(vstat.Blocks-vstat.Bavail),
		VaultFree:       int64(vstat.Bsize) * int64(vstat.Bavail),
		VaultUsed:       100 - 100*float32(vstat.Bavail)/float32(vstat.Blocks),
		VaultFS:         Fstype(uint64(vstat.Type)),
		CatalogCapacity: int64(cstat.Bsize) * int64(cstat.Blocks),
		CatalogBytes:    int64(cstat.Bsize) * int64(cstat.Blocks-vstat.Bavail),
		CatalogFree:     int64(cstat.Bsize) * int64(cstat.Bavail),
		CatalogUsed:     100 - 100*float32(cstat.Bavail)/float32(cstat.Blocks),
		CatalogFS:       Fstype(uint64(vstat.Type)),
	}

	pages.ExecuteTemplate(w, "DF", report)
}

func webvacuum(w http.ResponseWriter, r *http.Request) {
	args := []string{"vacuum"}
	cmd := remotecommand(args...)

	go cmd.Run()

	http.Redirect(w, r, "/tools/", http.StatusFound)
}
func webdryrun(w http.ResponseWriter, r *http.Request) {
	setDefaults()

	backup := NewBackup(cfg)
	backup.Start(defaultName, "dry-run")

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")

	files := []string{}
	backup.ForEach(func(f string) { files = append(files, f) })
	sort.Strings(files)

	for _, f := range files {
		fmt.Fprintln(w, f)
	}
}

func web() {
	listen := "localhost:8080"
	flag.StringVar(&listen, "listen", listen, "Address to listen to")
	flag.StringVar(&listen, "l", listen, "-listen")
	Setup()

	verbose = false // disable verbose mode when using web ui

	pages = pages.Funcs(template.FuncMap{
		"date":        DateExpander,
		"dateRFC1123": func(args ...interface{}) string { return DateFormat(time.RFC1123, args...) },
		"dateRFC3339": func(args ...interface{}) string { return DateFormat(time.RFC3339, args...) },
		"bytes":       BytesExpander,
		"status":      BackupStatus,
		"isserver":    cfg.IsServer,
		"now":         time.Now,
		"hostname":    os.Hostname,
		"basename":    func(args ...interface{}) string { return filepath.Base(args[0].(string)) },
		"executable": func(args ...interface{}) string {
			if m, ok := args[0].(int64); ok && m&0111 != 0 {
				return "T"
			} else {
				return "F"
			}
		},
	})

	setuptemplate(webparts)

	http.HandleFunc("/css/", stylesheets)
	http.HandleFunc("/info/", webinfo)
	http.HandleFunc("/list/", webinfo)
	http.HandleFunc("/backups/", webinfo)
	http.HandleFunc("/config/", webconfig)
	if cfg.IsServer() {
		http.HandleFunc("/tools/", webtools)
		http.HandleFunc("/tools/vacuum", webvacuum)
	}
	http.HandleFunc("/", webhome)
	http.HandleFunc("/about/", webhome)
	http.HandleFunc("/delete/", webdelete)
	http.HandleFunc("/new/", webnew)
	http.HandleFunc("/start/", webstart)
	http.HandleFunc("/dryrun/", webdryrun)
	webdavHandleFuncs()

	Info(false)
	Failure(false)
	timeout = 10
	if err := http.ListenAndServe(listen, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal("Could no start web interface: ", err)
	}
}
