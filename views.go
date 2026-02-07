package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"text/template"
)

func localPostFormHandler(w http.ResponseWriter, r *http.Request, s *site) {
	secret := r.Header.Get("X-Cask-Cluster-Secret")
	if !s.Cluster.CheckSecret(secret) {
		log.Println("unauthorized local file request")
		http.Error(w, "sorry, need the secret knock", 403)
		return
	}
	fmt.Fprintf(w, "show form/handle post\n")
	return
}

// read/write file requests that shall only touch
// the current node. No cluster interaction.
func localHandler(w http.ResponseWriter, r *http.Request, s *site) {
	secret := r.Header.Get("X-Cask-Cluster-Secret")
	if !s.Cluster.CheckSecret(secret) {
		log.Println("unauthorized local file request")
		http.Error(w, "sorry, need the secret knock", 403)
		return
	}
	key := r.PathValue("key")
	log.Printf("/local/%s/\n", key)
	k, err := keyFromString(key)
	if err != nil {
		http.Error(w, "invalid key\n", 400)
		return
	}
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if inm == "\""+key+"\"" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if !s.Backend.Exists(*k) {
		http.Error(w, "not found\n", 404)
		return
	}

	data, err := s.Backend.Read(*k)
	if err != nil {
		log.Println(err)
		http.Error(w, "error reading file", 500)
		return
	}
	w.Header().Set("Content-Type", "application/octet")
	w.Header().Set("ETag", "\""+key+"\"")
	w.Write(data)
	// kick off a background goroutine to do read-repair
	go func() {
		s.VerifyKey(*k)
		s.Rebalance(*k)
	}()
}

func handleLocalPost(w http.ResponseWriter, r *http.Request, s *site) {
	secret := r.Header.Get("X-Cask-Cluster-Secret")
	if !s.Cluster.CheckSecret(secret) {
		log.Println("unauthorized local file request")
		http.Error(w, "sorry, need the secret knock", 403)
		return
	}

	if r.ContentLength > s.MaxUploadSize {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	log.Println("write a file")
	if !s.Node.Writeable {
		http.Error(w, "this node is read-only", 503)
		return
	}
	f, _, _ := r.FormFile("file")
	defer f.Close()
	h := sha1.New()
	io.Copy(h, f)
	key, err := keyFromString("sha1:" + fmt.Sprintf("%x", h.Sum(nil)))
	if err != nil {
		http.Error(w, "bad hash", 500)
		return
	}
	if s.Backend.Exists(*key) {
		log.Println("already exists, don't need to do anything")
		fmt.Fprintf(w, "%s", key.String())
		return
	}
	f.Seek(0, 0)
	err = s.Backend.Write(*key, f)
	if err != nil {
		http.Error(w, "could not write file", 500)
		return
	}
	fmt.Fprintf(w, "%s", key.String())
	return
}



func fileHandler(w http.ResponseWriter, r *http.Request, s *site) {
	key := r.PathValue("key")
	log.Printf("/file/%s/\n", key)
	k, err := keyFromString(key)
	if err != nil {
		http.Error(w, "invalid key\n", 400)
		return
	}
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if inm == "\""+key+"\"" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if s.Backend.Exists(*k) {
		data, err := s.Backend.Read(*k)
		if err != nil {
			log.Println(err)
			http.Error(w, "error reading file", 500)
			return
		}
		w.Header().Set("Content-Type", "application/octet")
		w.Header().Set("ETag", "\""+key+"\"")
		w.Write(data)
		// kick off a background goroutine to do read-repair
		go func() {
			s.VerifyKey(*k)
			s.Rebalance(*k)
		}()
		return
	}

	data, err := s.Cluster.Retrieve(*k)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("ETag", "\""+key+"\"")
	w.Write(data)
}

type clusterInfoPage struct {
	Title     string
	Cluster   *cluster
	Myself    *node
	Neighbors []node
	Site      *site
	FreeSpace uint64
}

func clusterInfoHandler(w http.ResponseWriter, r *http.Request, s *site) {
	p := clusterInfoPage{
		Title:     "cluster status",
		Cluster:   s.Cluster,
		Myself:    s.Node,
		Neighbors: s.Cluster.NeighborsInclusive(),
		Site:      s,
	}
	t, _ := template.New("cluster").Parse(clusterTemplate)
	t.Execute(w, p)
}

var defaultReplication = 3
var minReplication = 1

type postResponse struct {
	Key     string `json:"key"`
	Success bool   `json:"success"`
}

func postFileHandler(w http.ResponseWriter, r *http.Request, s *site) {
	log.Println("add a file")

	if r.ContentLength > s.MaxUploadSize {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	f, _, _ := r.FormFile("file")
	defer f.Close()
	h := sha1.New()
	io.Copy(h, f)
	key, err := keyFromString("sha1:" + fmt.Sprintf("%x", h.Sum(nil)))
	if err != nil {
		log.Println(err)
		http.Error(w, "bad hash", 500)
		return
	}
	f.Seek(0, 0)
	success := s.Cluster.AddFile(*key, f, defaultReplication, minReplication)
	pr := postResponse{
		Key:     key.String(),
		Success: success,
	}
	b, err := json.Marshal(pr)
	if err != nil {
		http.Error(w, "json error", 500)
		return
	}
	w.Write(b)
}

func joinFormHandler(w http.ResponseWriter, r *http.Request, s *site) {
	w.Write([]byte(joinTemplate))
}

func joinHandler(w http.ResponseWriter, r *http.Request, s *site) {
	if r.FormValue("url") == "" {
		fmt.Fprint(w, "no url specified")
		return
	}
	u := r.FormValue("url")
	secret := r.FormValue("secret")
	if !s.Cluster.CheckSecret(secret) {
		log.Println("got an unauthorized join attempt")
		log.Println(secret)
		http.Error(w, "need to know the secret knock", 403)
		return
	}
	parts := strings.Split(u, ",")
	_, err := mlist.Join(parts)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
	fmt.Fprintf(w, "Added node")
}

func configHandler(w http.ResponseWriter, r *http.Request, s *site) {
	b, err := json.Marshal(s.Node)
	if err != nil {
		log.Println(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// just ignore this crap
}

const clusterTemplate = `
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
</head>
<body>
<div class="container">
<h1>Node: {{.Myself.UUID}}</h1>
<table class="table">
<tr><th>Backend</th><td>{{.Site.Backend}}</td></tr>
<tr><th>Free Space</th><td>{{.Site.Backend.FreeSpace}}</td></tr>
<tr><th>Base</th><td>{{.Myself.BaseURL}}</td></tr>
<tr><th>Writeable</th><td>{{.Myself.Writeable}}</td></tr>
<tr><th>Replication</th><td>{{.Site.Replication}}</td></tr>
<tr><th>Max Replication</th><td>{{.Site.MaxReplication}}</td></tr>
<tr><th>Active Anti-Entropy Interval</th><td>{{.Site.AAEInterval}} seconds</td></tr>
</table>
<h2>cluster status</h2>
<table class="table table-condensed table-striped">
<tr>
<th>UUID</th>
<th>Base</th>
<th>Writeable</th>
<th>Last Seen</th>
<th>Last Failed</th>
</tr>
{{range .Neighbors}}
{{if .LastSeen.IsZero}}
{{else}}
<tr {{if .Unhealthy}}class="danger"{{end}}>
<td>{{.UUID}}</td>
<td><a href="{{.BaseURL}}">{{.BaseURL}}</a></td>
<td>{{if .Writeable}}<span class="text-success">yes</span>{{else}}<span class="text-danger">read-only</span>{{end}}</td>
<td>{{.LastSeenFormatted}}</td>
<td>{{if .LastFailed.IsZero}}-{{else}}{{.LastFailedFormatted}}{{end}}</td>
</tr>
{{end}}
{{end}}
</table>
</body>

<ul class="nav nav-pills">
<li role="presentation"><a href="/join/">Add a node manually</a></li>
<li role="presentation"><a href="/config/">JSON config data</a></li>
</ul>

</div>
</html>
`

const joinTemplate = `
<html><head><title>Add Node</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
</head>
<body>
<div class="container">
<h1>Add Node</h1>
<form action="." method="post" class="form">
<input type="text" name="url" placeholder="Gossip Address" size="128" class="form-control"/><br />
<input type="text" name="secret" placeholder="cluster secret" class="form-control"/><br />
<input type="submit" value="add node" />
</form>
</div>
</body>
</html>
`
