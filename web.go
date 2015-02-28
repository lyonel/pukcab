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
	Name  string
	Major int
	Minor int
	OS    string
	Arch  string
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
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	report := &AboutReport{
		Report: Report{
			Title: programName + " on " + defaultName,
		},
		Name:  defaultName,
		Major: versionMajor,
		Minor: versionMinor,
		OS:    strings.ToTitle(runtime.GOOS[:1]) + runtime.GOOS[1:],
		Arch:  runtime.GOARCH,
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
		}
	}

	if name == "" && !IsServer() {
		name = defaultName
	}

	if name == "*" {
		name = ""
	}

	w.Header().Set("Refresh", "900")
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	args := []string{"metadata"}
	if name != "" {
		args = append(args, "-name", name)
	}
	args = append(args, flag.Args()...)
	cmd := remotecommand(args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
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
			fmt.Fprintln(w, "Backend error:", err)
			log.Println(err)
			return
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			var header BackupInfo
			dec := gob.NewDecoder(tr)
			if err := dec.Decode(&header); err != nil {
				fmt.Fprintln(w, "Protocol error:", err)
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
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

	if len(report.Backups) > 1 {
		for i, j := 0, len(report.Backups)-1; i < j; i, j = i+1, j-1 {
			report.Backups[i], report.Backups[j] = report.Backups[j], report.Backups[i]
		}
		if err := pages.ExecuteTemplate(w, "BACKUPS", report); err != nil {
			log.Println(err)
		}
	} else {
		report.Title = "Backup"
		if err := pages.ExecuteTemplate(w, "BACKUP", report); err != nil {
			log.Println(err)
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
		}
	}

	if date == 0 || name == "" {
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
		return
	}

	//w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	args := []string{"purgebackup", "-name", name, "-date", fmt.Sprintf("%d", date)}
	cmd := remotecommand(args...)

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

	go cmd.Wait()

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
		"isserver": IsServer,
		"now":      time.Now,
		"hostname": os.Hostname,
	})

	setuptemplate(webparts)
	setuptemplate(homepagetemplate)
	setuptemplate(configtemplate)
	setuptemplate(backupstemplate)
	setuptemplate(backuptemplate)
	setuptemplate(toolstemplate)

	http.HandleFunc("/css/", stylesheets)
	http.HandleFunc("/info/", webinfo)
	http.HandleFunc("/list/", webinfo)
	http.HandleFunc("/backups/", webinfo)
	http.HandleFunc("/config/", webconfig)
	http.HandleFunc("/tools/", webtools)
	http.HandleFunc("/", webhome)
	http.HandleFunc("/about/", webhome)
	http.HandleFunc("/delete/", webdelete)
	if err := http.ListenAndServe(listen, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal("Could no start web interface: ", err)
	}
}
