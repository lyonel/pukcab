package main

import (
	"encoding/gob"
	"ezix.org/tar"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

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

	w.Header().Set("Refresh", "3600")
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")

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
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
		return
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
			fmt.Fprintln(w, "Backend error:", err)
			log.Println(err)
			return
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			if !first {
				fmt.Fprintln(w)
			}
			first = false
			size = 0
			files = 0
			missing = 0

			var header BackupInfo
			dec := gob.NewDecoder(tr)
			if err := dec.Decode(&header); err != nil {
				fmt.Fprintln(w, "Protocol error:", err)
				log.Println(err)
				return
			}

			fmt.Fprintln(w, "Date:    ", header.Date)
			fmt.Fprintln(w, "Name:    ", header.Name)
			fmt.Fprintln(w, "Schedule:", header.Schedule)
			fmt.Fprintln(w, "Started: ", time.Unix(int64(header.Date), 0))
			if header.Finished.Unix() != 0 {
				fmt.Fprintln(w, "Finished:", header.Finished)
				fmt.Fprintln(w, "Duration:", header.Finished.Sub(time.Unix(int64(header.Date), 0)))
			}
			if header.Files > 0 {
				fmt.Fprintln(w, "Size:    ", Bytes(uint64(header.Size)))
				fmt.Fprintln(w, "Files:   ", header.Files)
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
				fmt.Fprintf(w, "%s %8s %-8s", hdr.FileInfo().Mode(), hdr.Uname, hdr.Gname)
				if s, err := strconv.ParseUint(hdr.Xattrs["backup.size"], 0, 0); err == nil {
					fmt.Fprintf(w, "%8s", Bytes(s))
				} else {
					fmt.Fprintf(w, "%8s", "")
				}
				fmt.Fprintf(w, " %s", hdr.Name)
				if hdr.Linkname != "." {
					fmt.Fprintf(w, " ➙ %s\n", hdr.Linkname)
				} else {
					fmt.Fprintln(w)
				}
			}
		}
	}
	if files > 0 {
		fmt.Fprint(w, "Complete: ")
		if files > 0 && missing > 0 {
			fmt.Fprintf(w, "%.1f%% (%d files missing)\n", 100*float64(files-missing)/float64(files), missing)
		} else {
			fmt.Fprintln(w, "yes")
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

}

func web() {
	listen := ":8080"
	flag.StringVar(&listen, "listen", listen, "Address to listen to")
	flag.StringVar(&listen, "l", listen, "-listen")
	Setup()

	http.HandleFunc("/info/", webinfo)
	http.HandleFunc("/list/", webinfo)
	if err := http.ListenAndServe(listen, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal("Could no start web interface: ", err)
	}
}
