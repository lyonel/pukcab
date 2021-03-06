package main

import (
	"fmt"
	"html/template"
	"log"
	"time"
)

const css = `body {
    font-family: sans-serif;
    font-weight: light;
    background: white;
}

.ls, tt {
    font-family: Inconsolata, Consolas, Monaco, "Andale Mono";
}

tt {
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

tr.daily {
}
tr.weekly {
    background-color: #ffe;
}
tr.monthly {
    background-color: #ffd;
}
tr.yearly {
    background-color: #ffc;
}
tr.total td.total {
    border-radius: 0;
    border-top: 1px solid #ccc;
}

.rowtitle {
    text-align: right;
}

.placeholder {
    font-size: .6em;
    color: #ccc;
    padding: 10em;
}

.starting {
    font-weight: bold;
    background-color: #efe;
}
.running {
    font-weight: bold;
}
.stuck {
    font-weight: bold;
    color: #ff7e00;
}
.failed {
    font-weight: bold;
    color: #f00;
}

#progress {
 width: 100px;   
 border: 1px solid #ccc;
 position: relative;
 padding: 1px;
}

.percent {
 position: absolute;   
 left: 50%;
 font-size: .2em;
}

#bar {
 height: 10px;
 background-color: #ccc;
 width: 50%;
}

.ls .date,
.ls .size,
.ls .user {
    text-align: right;
}

.ls .date {
 color: #bbb;
 background-color: #fbfbfb;
}

.ls .name {
}

.ls .type53 {
  color: #008;
}

.ls .type50 {
  color: #800;
}

`

const webparts = `{{define "MAINMENU"}}<div class="mainmenu">
<a href="{{root}}/">Home</a>
<a href="{{root}}/dashboard">Dashboard</a>
<a href="{{root}}/backups">Backups</a>
{{if isserver}}<a href="{{root}}/tools">Tools</a>{{end}}
</div>{{end}}
{{define "BROWSEMENU"}}
<div class="submenu">
<a class="label" href="..">&#x2191; Up</a>
</div>{{end}}
{{define "TOOLSMENU"}}
<div class="submenu">
<a class="label" href="{{root}}/tools/vacuum" onclick="return confirm('This may take a while and cannot be interrupted.\n\nAre you sure?')">Vacuum</a>
</div>{{end}}
{{define "HEADER"}}<!DOCTYPE html>
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="{{root}}/css/default.css">
<body>
<h1>{{.Title}}</h1>{{template "MAINMENU" .}}{{end}}
{{define "FOOTER"}}<div class="footer">{{now}}</div>
</body>
</html>{{end}}
{{define "HOMEMENU"}}
<div class="submenu">
<a class="label" href="{{root}}/about">About</a>
<a class="label" href="{{root}}/config">Configuration</a>
</div>{{end}}

{{define "HOME"}}{{template "HEADER" .}}
{{template "HOMEMENU" .}}
<table class="report"><tbody>
{{if .Name}}<tr><th class="rowtitle">Name</th><td>{{.Name}}</td><tr>{{end}}
{{if .Major}}<tr><th class="rowtitle">Pukcab</th><td title="{{.Build}}">{{.Major}}.{{.Minor}}</td></tr>{{end}}
{{if .OS}}<tr><th class="rowtitle">OS</th><td>{{.OS}}/{{if .Arch}}{{.Arch}}{{end}}</td></tr>{{end}}
{{if .CPUs}}<tr><th class="rowtitle">CPU(s)</th><td>{{.CPUs}}</td></tr>{{end}}
{{if .Load}}<tr><th class="rowtitle">Load</th><td>{{.Load}}</td></tr>{{end}}
{{if .Goroutines}}<tr><th class="rowtitle">Tasks</th><td>{{.Goroutines}}</td></tr>{{end}}
{{if .Bytes}}<tr><th class="rowtitle">Memory</th><td>{{.Memory | bytes}} ({{.Bytes | bytes}} used)</td></tr>{{end}}
</tbody></table>
{{template "FOOTER" .}}{{end}}

{{define "CONFIG"}}{{template "HEADER" .}}
{{template "HOMEMENU" .}}
<table class="report"><tbody>
{{if .Server}}
<tr><th class="rowtitle">Role</th><td>client</td></tr>
<tr><th class="rowtitle">Server</th><td>{{.Server}}</td></tr>
{{if .Port}}<tr><th class="rowtitle">Port</th><td>{{.Port}}</td></tr>{{end}}
{{if .Command}}<tr><th class="rowtitle">Command</th><td>{{.Command}}</td></tr>{{end}}
{{else}}
<tr><th class="rowtitle">Role</th><td>server</td></tr>
{{if .Vault}}<tr><th class="rowtitle">Vault</th><td><tt>{{.Vault}}</tt></td></tr>{{end}}
{{if .Catalog}}<tr><th class="rowtitle">Catalog</th><td><tt>{{.Catalog}}</tt></td></tr>{{end}}
{{if .Maxtries}}<tr><th class="rowtitle">Maxtries</th><td>{{.Maxtries}}</td></tr>{{end}}
<tr><th class="rowtitle">Expiration</th><td>
{{if .Expiration.Daily}}<tt>daily={{.Expiration.Daily}}</tt>{{end}}
{{if .Expiration.Weekly}}<tt>weekly={{.Expiration.Weekly}}</tt>{{end}}
{{if .Expiration.Monthly}}<tt>monthly={{.Expiration.Monthly}}</tt>{{end}}
{{if .Expiration.Yearly}}<tt>yearly={{.Expiration.Yearly}}</tt>{{end}}
</td></tr>
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
{{template "FOOTER" .}}{{end}}

{{define "DASHBOARD"}}{{template "HEADER" .}}
<div class="submenu">
{{if not isserver}}<a class="label" href="{{root}}/dashboard/">&#9733;</a>{{end}}
<a class="label" href="{{root}}/dashboard/*">All</a>
</div>
{{$me := hostname}}
{{$count := len .Clients}}
{{$schedules := .Schedules}}
<table class="report">
<thead><tr><th>Name</th><th>Backups</th><th>First backup</th>{{range .Schedules}}<th>Last {{.}}</th>{{end}}</tr></thead>
	{{with .Clients}}
<tbody>
    {{range .}}
        <td><a href="../backups/{{.Name}}">{{.Name}}</a>{{if eq .Name $me}} &#9734;{{end}}</td>
        <td title="{{.Size | bytes}}">{{.Count}}</td>
        <td title="{{.First | date}}"><a href="{{root}}/backups/{{.Name}}/{{.First.Unix}}">{{.First | dateshort}}</a></td>
        {{$last := .Last}}
        {{$name := .Name}}
	{{range $s := $schedules}}<td title="{{(index $last $s) | date}}"><a href="{{root}}/backups/{{$name}}/{{(index $last $s).Unix}}">{{(index $last $s) | dateshort}}</a></td>{{end}}
	</tr>
    {{end}}
{{end}}
</tbody>
</table>
    {{if not $count}}<div class="placeholder">empty list</div>{{end}}
{{template "FOOTER" .}}{{end}}

{{define "BACKUPS"}}{{template "HEADER" .}}
<div class="submenu">
{{if not isserver}}<a class="label" href="{{root}}/backups/">&#9733;</a>{{end}}
<a class="label" href="{{root}}/backups/*">All</a>
<a class="label" href="{{root}}/new/">New...</a>
</div>
{{$me := hostname}}
{{$count := len .Backups}}
	{{with .Backups}}
<table class="report">
<thead><tr><th>ID</th><th>Name</th><th>Schedule</th><th>Finished</th><th>Size</th></tr></thead>
<tbody>
    {{range .}}
	<tr class="{{. | status}} {{.Schedule}}">
        <td title="{{.Date | date}}"><a href="{{root}}/backups/{{.Name}}/{{.Date}}">{{.Date}}</a></td>
        <td title="{{. | status}}"><a href="{{.Name}}">{{.Name}}</a>{{if eq .Name $me}} &#9734;{{end}}</td>
        <td>{{.Schedule}}</td>
        <td {{if (.Finished | date)}} title="{{(.Finished.Sub .Date.Time)}}"{{end}}>{{.Finished | date}}</td>
        <td {{if .Files}}title="{{.Files}} files"{{end}}>{{if .Size}}{{.Size | bytes}}{{end}}</td>
	</tr>
    {{end}}
{{end}}
<tr class="total">
<td></td>
<td></td>
<td></td>
<td>Total</td>
<td class="total"{{if .Files}}title="{{.Files}} files"{{end}}>{{if .Size}}{{.Size | bytes}}{{end}}</td>
</tr>
</tbody>
</table>
    {{if not $count}}<div class="placeholder">empty list</div>{{end}}
{{template "FOOTER" .}}{{end}}

{{define "BACKUP"}}{{template "HEADER" .}}
{{with .Backups}}
{{$me := hostname}}
    {{range .}}
<div class="submenu">{{if .Files}}<a href="{{root}}/dav/{{.Name}}/{{.Date}}">Open</a><a href="" {{if ne $me .Name}}onclick="return confirm('This backup seems to be from a different system ({{.Name}}).\n\nAre you sure you want to verify it on {{$me}}?')"{{end}}>&#10003; Verify</a>{{end}}<a href="{{root}}/delete/{{.Name}}/{{.Date}}" onclick="return confirm('Are you sure?')" class="caution">&#10006; Delete</a></div>
<table class="report">
<tbody>
	<tr><th class="rowtitle">ID</th><td class="{{. | status}}" title="{{. | status}}">{{.Date}}</td></tr>
        <tr><th class="rowtitle">Name</th><td>{{.Name}}</td></tr>
        <tr class="{{.Schedule}}"><th class="rowtitle">Schedule</th><td>{{.Schedule}}</td></tr>
        <tr><th class="rowtitle">Started</th><td>{{.Date | date}}</td></tr>
        {{if .Files}}<tr><th class="rowtitle">Finished</th><td title="{{.Finished}}">{{.Finished | date}}</td></tr>{{end}}
        {{if .Size}}<tr><th class="rowtitle">Size</th><td>{{.Size | bytes}}</td></tr>{{end}}
        {{if .Files}}<tr><th class="rowtitle">Files</th><td>{{.Files}}</td></tr>{{end}}
</tbody>
</table>
    {{end}}
{{end}}
{{template "FOOTER" .}}{{end}}

{{define "NEW"}}{{template "HEADER" .}}
<div class="submenu"><a href="{{root}}/start/">Start</a></div>
<table class="report"><tbody>
<tr><th class="rowtitle">Name</th><td>{{hostname}}</td></tr>
{{if .Server}}
<tr><th class="rowtitle">Target</th><td>{{if .User}}{{.User}}@{{end}}{{.Server}}{{if .Port}}:{{.Port}}{{end}}</td></tr>
{{else}}
<tr><th class="rowtitle">Target</th><td>{{hostname}} (<em>self</em>)</td></tr>
{{end}}
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
{{template "FOOTER" .}}{{end}}

{{define "TOOLS"}}{{template "HEADER" .}}
{{template "TOOLSMENU"}}
<table class="report"><tbody>
</tbody></table>
{{template "FOOTER" .}}{{end}}

{{define "DF"}}{{template "HEADER" .}}
{{template "TOOLSMENU"}}
<table class="report"><tbody>
{{if .VaultCapacity}}<tr><th class="rowtitle">Storage{{if .CatalogCapacity}} (vault){{end}}</th><td>{{.VaultCapacity | bytes}}</td></tr>{{end}}
{{if .VaultFS}}<tr><th class="rowtitle">Filesystem</th><td>{{.VaultFS}}</td></tr>{{end}}
{{if .VaultBytes}}<tr><th class="rowtitle">Used</th><td><div id="progress" title="{{printf "%.1f" .VaultUsed}}%"><div id="bar" style="width:{{printf "%.1f" .VaultUsed}}%"></div></div></td></tr>{{end}}
{{if .VaultFree}}<tr><th class="rowtitle">Free</th><td>{{.VaultFree | bytes}}</td></tr>{{end}}
{{if .CatalogCapacity}}
<tr><th class="rowtitle">Storage (catalog)</th><td>{{.CatalogCapacity | bytes}}</td></tr>
{{if .CatalogFS}}<tr><th class="rowtitle">Filesystem</th><td>{{.CatalogFS}}</td></tr>{{end}}
{{if .CatalogBytes}}<tr><th class="rowtitle">Used</th><td><div id="progress"><div id="bar" style="width:{{printf "%.1f" .CatalogUsed}}%"></div></div></td></tr>{{end}}
{{if .CatalogFree}}<tr><th class="rowtitle">Free</th><td>{{.CatalogFree | bytes}}</td></tr>{{end}}
{{end}}
</td></tr>
</tbody></table>
{{template "FOOTER" .}}{{end}}

{{define "BUSY"}}{{template "HEADER" .}}
<div class="submenu">
<a class="label" href="{{root}}/">Cancel</a>
</div>
<div class="placeholder">Busy, retrying...</div>
{{template "FOOTER" .}}{{end}}

{{define "DAVROOT0"}}
<D:multistatus xmlns:D="DAV:" xmlns:P="http://pukcab.ezix.org/">
    <D:response>
        <D:href>/dav/</D:href>
	<D:propstat>
           <D:prop>
              <D:resourcetype><D:collection/></D:resourcetype>
           </D:prop>
           <D:status>HTTP/1.1 200 OK</D:status>
         </D:propstat>
    </D:response>
</D:multistatus>
{{end}}

{{define "DAVROOT"}}
<D:multistatus xmlns:D="DAV:" xmlns:P="http://pukcab.ezix.org/">
    <D:response>
        <D:href>/dav/.../</D:href>
	<D:propstat>
           <D:prop>
              <D:displayname>All backups...</D:displayname>
              <D:resourcetype><D:collection/></D:resourcetype>
           </D:prop>
           <D:status>HTTP/1.1 200 OK</D:status>
         </D:propstat>
    </D:response>
{{with .Names}}
    {{range .}}
    <D:response>
        <D:href>/dav/{{.}}/</D:href>
	<D:propstat>
           <D:prop>
              <D:resourcetype><D:collection/></D:resourcetype>
           </D:prop>
           <D:status>HTTP/1.1 200 OK</D:status>
         </D:propstat>
    </D:response>
    {{end}}
{{end}}
</D:multistatus>
{{end}}

{{define "BROWSEROOT"}}{{template "HEADER" .}}
{{template "BROWSEMENU"}}
<ul>
   <li><a href="{{root}}/dav/.../">All backups...</a></li>
{{range .Names}}
   <li><a href="{{root}}/dav/{{.}}/">{{.}}</a></li>
{{end}}
</ul>
{{template "FOOTER" .}}{{end}}

{{define "DAVBACKUPS0"}}
<D:multistatus xmlns:D="DAV:" xmlns:P="http://pukcab.ezix.org/">
    <D:response>
        <D:href>/dav/{{.}}/</D:href>
	<D:propstat>
           <D:prop>
              <D:resourcetype><D:collection/></D:resourcetype>
           </D:prop>
           <D:status>HTTP/1.1 200 OK</D:status>
         </D:propstat>
    </D:response>
</D:multistatus>
{{end}}

{{define "DAVBACKUPS"}}
<D:multistatus xmlns:D="DAV:" xmlns:P="http://pukcab.ezix.org/">
{{with .Backups}}
    {{range .}}
    {{if .Files}}
    <D:response>
        <D:href>/dav/{{.Name}}/{{.Date}}/</D:href>
	<D:propstat>
           <D:prop>
              <P:date>{{.Date}}</P:date>
              <P:finished>{{.Finished | dateRFC3339}}</P:finished>
              <P:schedule>{{.Schedule}}</P:schedule>
              <P:files>{{.Files}}</P:files>
              <P:size>{{.Size}}</P:size>
              <D:creationdate>{{.Date | dateRFC3339}}</D:creationdate>
              <D:getlastmodified>{{.Finished | dateRFC1123}}</D:getlastmodified>
              <D:getcontentlength>{{.Size}}</D:getcontentlength>
              <D:resourcetype><D:collection/></D:resourcetype>
           </D:prop>
           <D:status>HTTP/1.1 200 OK</D:status>
         </D:propstat>
    </D:response>
    {{end}}
    {{end}}
{{end}}
</D:multistatus>
{{end}}

{{define "BROWSEBACKUPS"}}{{template "HEADER" .}}
{{template "BROWSEMENU"}}
<ul>
{{range .Backups}}
   <li><a href="{{root}}/dav/{{.Name}}/{{.Date}}/">{{.Date}}</a></li>
{{end}}
</ul>
{{template "FOOTER" .}}{{end}}

{{define "DAVBACKUP0"}}
<D:multistatus xmlns:D="DAV:" xmlns:P="http://pukcab.ezix.org/">
    <D:response>
        <D:href>/dav/{{.Name}}/{{.Date}}/</D:href>
	<D:propstat>
           <D:prop>
              <P:date>{{.Date}}</P:date>
              <P:finished>{{.Finished | dateRFC3339}}</P:finished>
              <P:schedule>{{.Schedule}}</P:schedule>
              <P:files>{{.Files}}</P:files>
              <P:size>{{.Size}}</P:size>
              <D:creationdate>{{.Date | dateRFC3339}}</D:creationdate>
              <D:getlastmodified>{{.Finished | dateRFC1123}}</D:getlastmodified>
              <D:getcontentlength>{{.Size}}</D:getcontentlength>
              <D:resourcetype><D:collection/></D:resourcetype>
           </D:prop>
           <D:status>HTTP/1.1 200 OK</D:status>
         </D:propstat>
    </D:response>
</D:multistatus>
{{end}}

{{define "DAVBACKUP"}}
<D:multistatus xmlns:D="DAV:" xmlns:P="http://pukcab.ezix.org/" xmlns:A="http://apache.org/dav/props/">
{{$name := .Name}}
{{$date := .Date}}
{{with .Items}}
    {{range .}}
    <D:response>
        <D:href>/dav/{{$name}}/{{$date}}{{.Name}}</D:href>
	<D:propstat>
           <D:prop>
              <D:getlastmodified>{{.ModTime | dateRFC1123}}</D:getlastmodified>
              {{if eq .Typeflag '0'}}<D:getcontentlength>{{.Size}}</D:getcontentlength>{{end}}
              <D:resourcetype>{{if eq .Typeflag '5'}}<D:collection/>{{end}}{{if eq .Typeflag '2'}}<D:redirectref/>{{end}}</D:resourcetype>
              {{if eq .Typeflag '2'}}<D:displayname>{{.Name | basename}} → {{.Linkname}}</D:displayname><D:reftarget><D:href>{{.Xattrs.href}}</D:href></D:reftarget>{{end}}
              {{if .Uname}}<D:owner>{{.Uname}}</D:owner>{{end}}
              {{if .Gname}}<D:group>{{.Gname}}</D:group>{{end}}
              <A:executable>{{.Mode | executable}}</A:executable>
           </D:prop>
           <D:status>HTTP/1.1 200 OK</D:status>
        </D:propstat>
    </D:response>
    {{end}}
{{end}}
</D:multistatus>
{{end}}

{{define "BROWSEBACKUP"}}{{template "HEADER" .}}
{{template "BROWSEMENU"}}
{{$name := .Name}}
{{$date := .Date}}
<h2 title="{{.Date | date}}">{{.Date}}</h2>
<table>
{{range .Items}}
    <tr class="ls"><td>{{.FileInfo.Mode}}</td><td class="user" title="{{.Uid}}">{{.Uname}}</td><td class="group" title="{{.Gid}}">{{.Gname}}</td><td class="size" {{if .Size}}title="{{.Size}}"{{end}}>{{if .Size}}{{.Size | bytes}}{{end}}</td><td class="date"><a href="{{root}}/history/{{$name}}{{.Name}}">{{.ModTime | date}}</a></td><td>{{if eq .Typeflag '5'}}&#8227;{{end}}</td><td class="name type{{.Typeflag}}"><a title="{{.Name}}" {{if eq .Typeflag '5'}}href="{{root}}/dav/{{$name}}/{{$date}}{{.Name}}/"{{end}}>{{.Name | basename}}</a>{{if eq .Typeflag '2'}} → <a href="{{.Xattrs.href}}/">{{.Linkname}}</a>{{end}}</td></tr>
{{end}}
</table>
{{template "FOOTER" .}}{{end}}

`

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

	now := time.Now().Local()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch duration := time.Since(t); {
	case t.After(midnight):
		return t.Format("today 15:04")
	case t.After(midnight.AddDate(0, 0, -1)):
		return t.Format("yesterday 15:04")
	case duration < 7*24*time.Hour:
		return t.Format("Monday 15:04")
	case duration < 365*24*time.Hour:
		return t.Format("Mon 2 January 15:04")
	}

	return t.Format("2 Jan 2006 15:04")
}

func DateShort(args ...interface{}) string {
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

	now := time.Now().Local()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch duration := time.Since(t); {
	case t.After(midnight):
		return t.Format("today")
	case t.After(midnight.AddDate(0, 0, -1)):
		return t.Format("yesterday")
	case duration < 7*24*time.Hour:
		return t.Format("Monday")
	case duration < 365*24*time.Hour:
		return t.Format("2 January")
	}

	return t.Format("2 Jan 2006")
}

func DateFormat(format string, args ...interface{}) string {
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

	return t.UTC().Format(format)
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

func BackupStatus(i BackupInfo) string {
	if !i.Finished.IsZero() && i.Finished.Unix() != 0 {
		return "finished"
	}

	switch duration := time.Since(i.LastModified); {
	case duration < 3*time.Minute:
		return "starting"
	case duration < time.Hour:
		return "running"
	case duration < 2*time.Hour:
		return "stuck"
	}

	return "failed"
}

func setuptemplate(s string) {
	var err error

	pages, err = pages.Parse(s)
	if err != nil {
		log.Println(err)
		log.Fatal("Could no start web interface: ", err)
	}
}
