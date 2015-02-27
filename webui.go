package main

import (
	"fmt"
	"html/template"
	"log"
	"time"
)

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

const webparts = `{{define "MAINMENU"}}<div class="mainmenu">
<a href="/">Home</a>
<a href="/backups">Backups</a>
</div>{{end}}
{{define "HEADER"}}<!DOCTYPE html>
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="/css/default.css">
<body>
<h1>{{.Title}}</h1>{{template "MAINMENU" .}}{{end}}
{{define "FOOTER"}}<div class="footer">{{.Date}}</div>
</body>
</html>{{end}}`

const homepagetemplate = `{{define "HOME"}}{{template "HEADER" .}}
<div class="submenu">
<a class="label" href="/about">About</a>
<a class="label" href="/config">Configuration</a>
</div>
<table class="report"><tbody>
{{if .Name}}<tr><th class="rowtitle">Name</th><td>{{.Name}}</td></tr>{{end}}
{{if .Major}}<tr><th class="rowtitle">Pukcab</th><td>{{.Major}}.{{.Minor}}</td></tr>{{end}}
{{if .OS}}<tr><th class="rowtitle">OS</th><td>{{.OS}}/{{if .Arch}}{{.Arch}}{{end}}</td></tr>{{end}}
</tbody></table>
{{template "FOOTER" .}}{{end}}`

const configtemplate = `{{define "CONFIG"}}{{template "HEADER" .}}
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
{{template "FOOTER" .}}{{end}}`

const backupstemplate = `{{define "BACKUPS"}}{{template "HEADER" .}}
<div class="submenu">
<a class="label" href="/backups/">&#9733;</a>
<a class="label" href="/backups/*">All</a>
</div>
{{$me := .Me}}
	{{with .Backups}}
<table class="report">
<thead><tr><th>ID</th><th>Name</th><th>Schedule</th><th>Finished</th><th>Size</th><th>Files</th></tr></thead>
<tbody>
    {{range .}}
	<tr>
        <td><a href="/backups/{{.Name}}/{{.Date}}">{{.Date}}</a></td>
        <td><a href="{{.Name}}">{{.Name}}</a>{{if eq .Name $me}} &#9734;{{end}}</td>
        <td>{{.Schedule}}</td>
        <td>{{.Finished | date}}</td>
        <td>{{if .Size}}{{.Size | bytes}}{{end}}</td>
        <td>{{if .Files}}{{.Files}}{{end}}</td>
	</tr>
    {{end}}
</tbody>
</table>
{{end}}
{{template "FOOTER" .}}{{end}}`

const backuptemplate = `{{define "BACKUP"}}{{template "HEADER" .}}
{{with .Backups}}
    {{range .}}
<div class="submenu">{{if .Files}}<a href="/files/{{.Date}}">Open</a><a href="/verify/{{.Date}}">&#10003; Verify</a>{{end}}<a href="/delete/{{.Date}}" class="caution">&#10006; Delete</a></div>
<table class="report">
<tbody>
	<tr><th class="rowtitle">ID</th><td>{{.Date}}</td></tr>
        <tr><th class="rowtitle">Name</th><td>{{.Name}}</td></tr>
        <tr><th class="rowtitle">Schedule</th><td>{{.Schedule}}</td></tr>
        <tr><th class="rowtitle">Started</th><td>{{.Date | date}}</td></tr>
        <tr><th class="rowtitle">Finished</th><td>{{.Finished | date}}</td></tr>
        {{if .Size}}<tr><th class="rowtitle">Size</th><td>{{.Size | bytes}}</td></tr>{{end}}
        {{if .Files}}<tr><th class="rowtitle">Files</th><td>{{.Files}}</td></tr>{{end}}
</tbody>
</table>
    {{end}}
{{end}}
{{template "FOOTER" .}}{{end}}`

var pages = template.New("webpages")

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

func setuptemplate(s string) {
	var err error

	pages, err = pages.Parse(s)
	if err != nil {
		log.Println(err)
		log.Fatal("Could no start web interface: ", err)
	}
}
