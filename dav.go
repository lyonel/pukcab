package main

import (
	"encoding/gob"
	"ezix.org/tar"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func listbackups(name string) (*BackupsReport, error) {
	args := []string{"metadata"}
	if name != "" {
		args = append(args, "-name", name)
	}
	cmd := remotecommand(args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(cmd.Args, err)
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		log.Println(cmd.Args, err)
		return nil, err
	}

	report := &BackupsReport{}

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
			return nil, err
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			var header BackupInfo
			dec := gob.NewDecoder(tr)
			if err := dec.Decode(&header); err != nil {
				log.Println(err)
				return nil, err
			} else {
				report.Backups = append(report.Backups, header)
				report.Size += header.Size
				report.Files += header.Files
				names[header.Name] = struct{}{}
				schedules[header.Schedule] = struct{}{}
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
				if status.ExitStatus() != 2 {
					log.Println(cmd.Args, err)
				}
			}
		} else {
			log.Println(cmd.Args, err)
		}

		return nil, err
	}

	return report, nil
}

func davroot(w http.ResponseWriter, r *http.Request) {
	for r.RequestURI[len(r.RequestURI)-1] == '/' {
		r.RequestURI = r.RequestURI[0 : len(r.RequestURI)-1]
	}
	if req := strings.SplitN(r.RequestURI[1:], "/", 3); len(req) > 1 {
		switch len(req) {
		case 2:
			if name = req[1]; name == "..." {
				name = ""
			}
			if name != "" && name[0] == '.' {
				http.Error(w, "Invalid request", http.StatusNotFound)
				return
			}

			davname(w, r)

		case 3:
			if d, err := strconv.Atoi(req[2]); err == nil {
				date = BackupID(d)
			} else {
				http.Error(w, "Invalid request", http.StatusNotAcceptable)
				return
			}
			if name = req[1]; name == "..." {
				name = ""
			}
			if name != "" && name[0] == '.' {
				http.Error(w, "Invalid request", http.StatusNotFound)
				return
			}

			davdate(w, r)
		}
		return
	}

	switch r.Method {
	case "OPTIONS":
		w.Header().Set("Allow", "GET, DELETE, PROPFIND")
		w.Header().Set("DAV", "1,2")

	case "PROPFIND":
		if report, err := listbackups(""); err == nil {
			w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>`))
			if err := pages.ExecuteTemplate(w, "DAVROOT", report); err != nil {
				log.Println(err)
				http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
			}
		} else {
			log.Println(err)
			http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		}

	default:
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
	}
}

func davname(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "OPTIONS":
		w.Header().Set("Allow", "GET, DELETE, PROPFIND")
		w.Header().Set("DAV", "1,2")

	case "PROPFIND":
		if report, err := listbackups(name); err == nil {
			w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>`))
			if err := pages.ExecuteTemplate(w, "DAVDOTDOTDOT", report); err != nil {
				log.Println(err)
				http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
			}
		} else {
			log.Println(err)
			http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		}

	default:
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
	}
}

func davdate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "OPTIONS":
		w.Header().Set("Allow", "GET, DELETE, PROPFIND")
		w.Header().Set("DAV", "1,2")

	default:
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
	}
}

func webdavHandleFuncs() {
	http.HandleFunc("/dav/", davroot)
}
