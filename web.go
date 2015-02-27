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
	Date  time.Time
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

const css = `
body {
    font-family: "Open Sans", Verdana, Geneva, Arial, Helvetica, sans-serif ;
    font-weight: normal;
}

tt {
    font-size: 1.2em;
    background: #eee;
    border: 1px solid #ccc;
    padding: 1px;
}

a:link {
    text-decoration: none;
    color: inherit;
}

a:visited {
    text-decoration: none;
    color: inherit;
}

a:hover {
    text-decoration: none;
    color: #CC704D;
}

a:active {
    text-decoration: none;
    color: #FF0000;
}

.mainmenu {
    font-size:.8em;
    clear:both;
    padding:10px;
    background:#eaeaea linear-gradient(#fafafa, #eaeaea) repeat-x;
    border:1px solid #eaeaea;
    border-radius:5px;
}

.mainmenu a {
    padding: 10px 20px;
    text-decoration:none;
    color: #777;
    border-right:1px solid #eaeaea;
}
.mainmenu a.active,
.mainmenu a:hover {
    color: #000;
    border-bottom:2px solid #D26911;
}

.submenu {
    font-size: .7em;
    margin-top: 10px;
    padding: 10px;
    border-bottom: 1px solid #ccc;
}

.caution:hover {
    background-color: #f66;
}

.submenu a {
    padding: 10px 11px;
    text-decoration:none;
    color: #777;
}

.submenu a:hover {
    padding: 6px 10px;
    border: 1px solid #ccc;
    border-radius: 5px;
    color: #000;
}

.footer {
    border-top: 1px solid #ccc;
    padding: 10px;
    font-size:.7em;
    margin-top: 10px;
    color: #ccc;
}

table.report {
    cursor: auto;
    border-radius: 5px;
    border: 1px solid #ccc;
    margin: 1em 0;
}
.report td, .report th {
   border: 0;
   font-size: .8em;
   padding: 10px;
}
.report td:first-child {
    border-top-left-radius: 5px;
}
.report tbody tr:last-child td:first-child {
    border-bottom-left-radius: 5px;
}
.report td:last-child {
    border-top-right-radius: 5px;
}
.report tbody tr:last-child {
    border-bottom-left-radius: 5px;
    border-bottom-right-radius: 5px;
}
.report tbody tr:last-child td:last-child {
    border-bottom-right-radius: 5px;
}
.report thead+tbody tr:hover {
    background-color: #e5e9ec !important;
}
.rowtitle {
    text-align: right;
}
`

const mainmenu = `<div class="mainmenu">
<a href="/">Home</a>
<a href="/backups">Backups</a>
</div>`

const templateheader = `<!DOCTYPE html>
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="/css/default.css">
<body>
<h1>{{.Title}}</h1>` +
	mainmenu

const templatefooter = `<div class="footer">{{.Date}}</div>
</body>
</html>`

const homepagetemplate = templateheader + `
<div class="submenu">
<a class="label" href="/about">About</a>
<a class="label" href="/config">Configuration</a>
</div>
<table class="report"><tbody>
{{if .Name}}<tr><th class="rowtitle">Name</th><td>{{.Name}}</td></tr>{{end}}
{{if .Major}}<tr><th class="rowtitle">Pukcab</th><td>{{.Major}}.{{.Minor}}</td></tr>{{end}}
{{if .OS}}<tr><th class="rowtitle">OS</th><td>{{.OS}}/{{if .Arch}}{{.Arch}}{{end}}</td></tr>{{end}}
</tbody></table>
` +
	templatefooter

const configtemplate = templateheader + `
<div class="submenu">
<a class="label" href="/about">About</a>
<a class="label" href="/config">Configuration</a>
</div>
<table class="report"><tbody>
{{if .Server}}
<tr><th class="rowtitle">Role</th><td>client</td></tr>
<tr><th class="rowtitle">Server</th><td>{{.Server}}</td></tr>
{{if .Port}}<tr><th class="rowtitle">Port</th><td>{{.Port}}</td></tr>{{end}}
{{else}}
<tr><th class="rowtitle">Role</th><td>server</td></tr>
{{if .Vault}}<tr><th class="rowtitle">Vault</th><td><tt>{{.Vault}}</tt></td></tr>{{end}}
{{if .Catalog}}<tr><th class="rowtitle">Catalog</th><td><tt>{{.Catalog}}</tt></td></tr>{{end}}
{{if .Maxtries}}<tr><th class="rowtitle">Maxtries</th><td>{{.Maxtries}}</td></tr>{{end}}
{{end}}
{{if .User}}<tr><th class="rowtitle">User</th><td>{{.User}}</td></tr>{{end}}
<tr><th class="rowtitle">Include</th><td>
{{range .Include}}
<tt>{{.}}</tt>
{{end}}
</td></tr>
<tr><th class="rowtitle">Exclude</th><td>
{{range .Exclude}}
<tt>{{.}}</tt>
{{end}}
</td></tr>
</tbody></table>
` +
	templatefooter

const backupstemplate = templateheader +
	`<div class="submenu">
<a class="label" href="/backups/">Default</a>
<a class="label" href="/backups/*">All</a>
</div>
	{{with .Backups}}
<table class="report">
<thead><tr><th>ID</th><th>Name</th><th>Schedule</th><th>Finished</th><th>Size</th><th>Files</th></tr></thead>
<tbody>
    {{range .}}
	<tr>
        <td><a href="/backups/{{.Name}}/{{.Date}}">{{.Date}}</a></td>
        <td><a href="{{.Name}}">{{.Name}}</a></td>
        <td>{{.Schedule}}</td>
        <td>{{.Finished | date}}</td>
        <td>{{if .Size}}{{.Size | bytes}}{{end}}</td>
        <td>{{if .Files}}{{.Files}}{{end}}</td>
	</tr>
    {{end}}
</tbody>
</table>
{{end}}` +
	templatefooter

const backuptemplate = templateheader +
	`<div class="submenu">
<a class="caution" href=".">Delete</a>
</div>
	{{with .Backups}}
<table class="report">
<tbody>
    {{range .}}
	<tr><th class="rowtitle">ID</th><td>{{.Date}}</td></tr>
        <tr><th class="rowtitle">Name</th><td>{{.Name}}</td></tr>
        <tr><th class="rowtitle">Schedule</th><td>{{.Schedule}}</td></tr>
        <tr><th class="rowtitle">Started</th><td>{{.Date | date}}</td></tr>
        <tr><th class="rowtitle">Finished</th><td>{{.Finished | date}}</td></tr>
        {{if .Size}}<tr><th class="rowtitle">Size</th><td>{{.Size | bytes}}</td></tr>{{end}}
        {{if .Files}}<tr><th class="rowtitle">Files</th><td>{{.Files}}</td></tr>{{end}}
    {{end}}
</tbody>
</table>
{{end}}` +
	templatefooter

var homepage, configpage, backupspage, backuppage template.Template

func DateExpander(args ...interface{}) string {
	ok := false
	var t time.Time
	if len(args) == 1 {
		t, ok = args[0].(time.Time)

		if !ok {
			var d BackupID
			if d, ok = args[0].(BackupID); ok {
				t = time.Unix(int64(d), 0)
			}
		}
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

func webhome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	report := &AboutReport{
		Report: Report{
			Title: programName + " on " + defaultName,
			Date:  time.Now(),
		},
		Name:  defaultName,
		Major: versionMajor,
		Minor: versionMinor,
		OS:    strings.ToTitle(runtime.GOOS[:1]) + runtime.GOOS[1:],
		Arch:  runtime.GOARCH,
	}
	homepage.Execute(w, report)
}

func webconfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	report := &ConfigReport{
		Report: Report{
			Title: programName + " on " + defaultName,
			Date:  time.Now(),
		},
		Config: cfg,
	}
	configpage.Execute(w, report)
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
		backupspage.Execute(w, report)
	} else {
		report.Title = "Backup"
		backuppage.Execute(w, report)
	}
}

func setuptemplate(s string) template.Template {
	t := template.New("webpage template")
	t = t.Funcs(template.FuncMap{"date": DateExpander})
	t = t.Funcs(template.FuncMap{"bytes": BytesExpander})
	t, err := t.Parse(s)
	if err != nil {
		log.Println(err)
		log.Fatal("Could no start web interface: ", err)
	}
	return *t
}

func web() {
	listen := ":8080"
	flag.StringVar(&listen, "listen", listen, "Address to listen to")
	flag.StringVar(&listen, "l", listen, "-listen")
	Setup()

	verbose = false // disable verbose mode when using web ui

	homepage = setuptemplate(homepagetemplate)
	configpage = setuptemplate(configtemplate)
	backupspage = setuptemplate(backupstemplate)
	backuppage = setuptemplate(backuptemplate)

	http.HandleFunc("/css/", stylesheets)
	http.HandleFunc("/info/", webinfo)
	http.HandleFunc("/list/", webinfo)
	http.HandleFunc("/backups/", webinfo)
	http.HandleFunc("/config/", webconfig)
	http.HandleFunc("/", webhome)
	http.HandleFunc("/about", webhome)
	if err := http.ListenAndServe(listen, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal("Could no start web interface: ", err)
	}
}
