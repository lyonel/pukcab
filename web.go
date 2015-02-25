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
	"strconv"
	"strings"
	"time"
)

type Report struct {
	Title string
	Date  time.Time
}

type InfoReport struct {
	Report
	Backups []BackupInfo
}

const css = `
body {
    font-family: Roboto, Verdana, Geneva, Arial, Helvetica, sans-serif ;
    font-weight: normal;
    font-size: 10px;
}

table
{
    font-size: 12px;
    text-align: center;
    color: #fff;
    background-color: #666;
    border: 0px;
    border-collapse: collapse;
    border-spacing: 0px;
}

table td
{
    color: #000;
    padding: 4px;
    border: 1px #fff solid;
}

tr:nth-child(even) { background: #EEE; }
tr:nth-child(odd) { background: #DDD; }

table th
{
    background-color: #666;
    color: #fff;
    padding: 4px;
    border-bottom: 2px #fff solid;
    font-size: 12px;
    font-weight: bold;
}
`

const infotemplate = `
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="/css/default.css">
<body>
{{with .Backups}}
<table>
<tr><th>ID</th><th>Name</th><th>Schedule</th><th>Finished</th><th>Size</th><th>Files</th></tr>
    {{range .}}
	<tr>
        <td><a href="/backup/{{.Date}}">{{.Date}}</a></td>
        <td><a href="/info/{{.Name}}">{{.Name}}</a></td>
        <td>{{.Schedule}}</td>
        <td>{{.Finished | date}}</td>
        <td>{{if .Size}}{{.Size | bytes}}{{end}}</td>
        <td>{{if .Files}}{{.Files}}{{end}}</td>
	</tr>
    {{end}}
</table>
{{end}}
<hr>
{{.Date}}
</body>
</html>
`

func DateExpander(args ...interface{}) string {
	ok := false
	var t time.Time
	if len(args) == 1 {
		t, ok = args[0].(time.Time)
	}
	if !ok {
		return fmt.Sprint(args...)
	}

	if t.IsZero() || t.Unix() == 0 {
		return ""
	}

	return t.Format(time.RFC822)
}

func BytesExpander(args ...interface{}) string {
	ok := false
	var n int64
	if len(args) == 1 {
		n, ok = args[0].(int64)
	}
	if !ok {
		return fmt.Sprint(args...)
	}

	return Bytes(uint64(n))
}

func stylesheets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=UTF-8")
	fmt.Fprintf(w, css)
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

	report := &InfoReport{
		Report: Report{
			Title: "Info",
			Date:  time.Now(),
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
				report.Backups = append(report.Backups, header)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Fprintln(w, "Backend error:", err)
		log.Println(cmd.Args, err)
		return
	}

	t := template.New("Info template")
	t = t.Funcs(template.FuncMap{"date": DateExpander})
	t = t.Funcs(template.FuncMap{"bytes": BytesExpander})
	if t, err := t.Parse(infotemplate); err == nil {
		t.Execute(w, report)
	} else {
		log.Println(err)
	}
}

func web() {
	listen := ":8080"
	flag.StringVar(&listen, "listen", listen, "Address to listen to")
	flag.StringVar(&listen, "l", listen, "-listen")
	Setup()

	http.HandleFunc("/css/", stylesheets)
	http.HandleFunc("/info/", webinfo)
	http.HandleFunc("/list/", webinfo)
	if err := http.ListenAndServe(listen, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal("Could no start web interface: ", err)
	}
}
