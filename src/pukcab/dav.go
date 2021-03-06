package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pukcab/tar"
)

// StatusMulti represents HTTP 207
const StatusMulti = 207

// DAVOptions lists the default options we return
const DAVOptions = "1, 2, access-control, redirectrefs"

// FilesReport lists files under a given path
type FilesReport struct {
	Report
	Date           BackupID
	Finished       time.Time
	Name, Schedule string
	Files, Size    int64
	Path           string
	Items          []*tar.Header
}

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

func listfiles(date BackupID, path string) (*FilesReport, error) {
	args := []string{"metadata", "-depth", "1", "-date", fmt.Sprintf("%d", date), path}
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

	report := &FilesReport{Path: path}

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
				report.Date = header.Date
				report.Size = header.Size
				report.Files = header.Files
				report.Name = header.Name
				report.Schedule = header.Schedule
			}
		case tar.TypeSymlink, tar.TypeLink:
			unfold(hdr)
			if hdr.Xattrs == nil {
				hdr.Xattrs = make(map[string]string)
			}
			if filepath.IsAbs(hdr.Linkname) {
				hdr.Xattrs["href"] = fmt.Sprintf("/dav/%s/%d%s", report.Name, report.Date, hdr.Linkname)
			} else {
				hdr.Xattrs["href"] = hdr.Linkname
			}
			report.Items = append(report.Items, hdr)
		default:
			unfold(hdr)
			report.Items = append(report.Items, hdr)
		}
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
	r.RequestURI = path.Clean(r.RequestURI)
	for r.RequestURI[len(r.RequestURI)-1] == '/' {
		r.RequestURI = r.RequestURI[0 : len(r.RequestURI)-1]
	}
	if req := strings.SplitN(r.RequestURI[1:], "/", 4); len(req) > 1 {
		switch {
		case len(req) == 2:
			if name = req[1]; name == "..." {
				name = ""
			}
			if (name != "" && name[0] == '.') || (strings.ToLower(name) == "desktop.ini") {
				http.Error(w, "Invalid request", http.StatusNotFound)
				return
			}

			davname(w, r)

		case len(req) >= 3:
			if d, err := strconv.Atoi(req[2]); err == nil {
				date = BackupID(d)
			} else {
				http.Error(w, "Invalid request", http.StatusNotFound)
				return
			}
			if name = req[1]; name == "..." {
				name = ""
			}
			if name != "" && name[0] == '.' {
				http.Error(w, "Invalid request", http.StatusNotFound)
				return
			}

			davbrowse(w, r)
		}
		return
	}

	switch r.Method {
	case "GET":
		if report, err := listbackups(""); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			if err := pages.ExecuteTemplate(w, "BROWSEROOT", report); err != nil {
				log.Println(err)
			}
		}

	case "OPTIONS", "HEAD":
		w.Header().Set("Allow", "GET, PROPFIND")
		w.Header().Set("DAV", DAVOptions)

	case "PROPFIND":
		if r.Header.Get("Depth") == "0" {
			w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
			w.WriteHeader(StatusMulti)
			if err := pages.ExecuteTemplate(w, "DAVROOT0", struct{}{}); err != nil {
				log.Println(err)
			}
		} else {
			if report, err := listbackups(""); err == nil {
				w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
				w.WriteHeader(StatusMulti)
				if err := pages.ExecuteTemplate(w, "DAVROOT", report); err != nil {
					log.Println(err)
				}
			} else {
				log.Println(err)
				http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
			}
		}

	case "PUT":
	case "DELETE":
	case "PATCH":
		http.Error(w, "Access denied.", http.StatusForbidden)

	default:
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
	}
}

func davname(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "OPTIONS", "HEAD":
		w.Header().Set("Allow", "GET, PROPFIND")
		w.Header().Set("DAV", DAVOptions)

	case "PROPFIND":
		if r.Header.Get("Depth") == "0" {
			w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
			w.WriteHeader(StatusMulti)
			if err := pages.ExecuteTemplate(w, "DAVBACKUPS0", name); err != nil {
				w.WriteHeader(StatusMulti)
				log.Println(err)
			}
		} else {
			if report, err := listbackups(name); err == nil {
				w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
				w.WriteHeader(StatusMulti)
				if err := pages.ExecuteTemplate(w, "DAVBACKUPS", report); err != nil {
					log.Println(err)
				}
			} else {
				log.Println(err)
				http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
			}
		}

	case "GET":
		if report, err := listbackups(name); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			if err := pages.ExecuteTemplate(w, "BROWSEBACKUPS", report); err != nil {
				log.Println(err)
			}
		}

	default:
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
	}
}

func davbrowse(w http.ResponseWriter, r *http.Request) {
	req := strings.SplitN(r.RequestURI[1:], "/", 4)
	if len(req) < 4 {
		req = append(req, "")
	}
	switch r.Method {
	case "OPTIONS", "HEAD":
		w.Header().Set("Allow", "GET, PROPFIND")
		w.Header().Set("DAV", DAVOptions)

	case "PROPFIND":
		if report, err := listfiles(date, "/"+req[3]); err == nil {
			if r.Header.Get("Depth") == "0" && req[3] == "" {
				if len(report.Items) == 0 {
					http.Error(w, "Not found.", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
				w.WriteHeader(StatusMulti)
				if err := pages.ExecuteTemplate(w, "DAVBACKUP0", report); err != nil {
					log.Println(err)
				}
			} else {
				if len(report.Items) == 0 {
					http.Error(w, "Not found.", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/xml; charset=UTF-8")
				w.WriteHeader(StatusMulti)
				if err := pages.ExecuteTemplate(w, "DAVBACKUP", report); err != nil {
					log.Println(err)
				}
			}
		} else {
			log.Println(err)
			http.Error(w, "Internal error: "+err.Error(), http.StatusInternalServerError)
		}

	case "GET":
		if report, err := listfiles(date, "/"+req[3]); err == nil {
			report.Title = name + ":/" + req[3]
			if len(report.Items) > 0 && len(req[3])+1 >= len(report.Items[0].Name) {
				report.Items = report.Items[1:]
			}
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			if err := pages.ExecuteTemplate(w, "BROWSEBACKUP", report); err != nil {
				log.Println(err)
			}
		}

	default:
		http.Error(w, "Invalid request", http.StatusNotAcceptable)
	}
}

func webdavHandleFuncs() {
	http.HandleFunc("/dav/", davroot)
	http.HandleFunc("/DAV/", davroot)
	http.HandleFunc("/WebDAV/", davroot)
	http.HandleFunc("/webdav/", davroot)
}
