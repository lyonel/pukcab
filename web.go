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
	"runtime"
	"strconv"
	"strings"
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
	Backups []BackupInfo
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
		Backups: []BackupInfo{},
	}

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
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Println(cmd.Args, err)
		http.Error(w, "Backend error: "+err.Error(), http.StatusServiceUnavailable)
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

func webtools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	report := &ConfigReport{
		Report: Report{
			Title: programName + " on " + defaultName,
		},
		Config: cfg,
	}
	pages.ExecuteTemplate(w, "TOOLS", report)
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

	if err := cmd.Start(); err != nil {
		log.Println(cmd.Args, err)
		http.Error(w, "Backend error: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	go cmd.Wait()

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

func web() {
	listen := ":8080"
	flag.StringVar(&listen, "listen", listen, "Address to listen to")
	flag.StringVar(&listen, "l", listen, "-listen")
	Setup()

	verbose = false // disable verbose mode when using web ui

	pages = pages.Funcs(template.FuncMap{
		"date":     DateExpander,
		"bytes":    BytesExpander,
		"status":   BackupStatus,
		"isserver": cfg.IsServer,
		"now":      time.Now,
		"hostname": os.Hostname,
	})

	setuptemplate(webparts)
	setuptemplate(homepagetemplate)
	setuptemplate(configtemplate)
	setuptemplate(backupstemplate)
	setuptemplate(backuptemplate)
	setuptemplate(toolstemplate)
	setuptemplate(newtemplate)

	http.HandleFunc("/css/", stylesheets)
	http.HandleFunc("/info/", webinfo)
	http.HandleFunc("/list/", webinfo)
	http.HandleFunc("/backups/", webinfo)
	http.HandleFunc("/config/", webconfig)
	if cfg.IsServer() {
		http.HandleFunc("/tools/", webtools)
	}
	http.HandleFunc("/", webhome)
	http.HandleFunc("/about/", webhome)
	http.HandleFunc("/delete/", webdelete)
	http.HandleFunc("/new/", webnew)
	http.HandleFunc("/start/", webstart)
	if err := http.ListenAndServe(listen, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal("Could no start web interface: ", err)
	}
}
