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
	"time"
)

// read/write file requests that shall only touch
// the current node. No cluster interaction.
func localHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	secret := r.Header.Get("X-Cask-Cluster-Secret")
	if !s.Cluster.CheckSecret(secret) {
		log.Println("unauthorized local file request")
		http.Error(w, "sorry, need the secret knock", 403)
		return
	}
	parts := strings.Split(r.URL.String(), "/")
	if len(parts) == 3 {
		if r.Method == "POST" {
			log.Println("write a file")
			if !s.Node.Writeable {
				http.Error(w, "this node is read-only", 503)
				return
			}
			f, _, _ := r.FormFile("file")
			defer f.Close()
			h := sha1.New()
			io.Copy(h, f)
			key, err := KeyFromString("sha1:" + fmt.Sprintf("%x", h.Sum(nil)))
			if err != nil {
				http.Error(w, "bad hash", 500)
				return
			}
			if s.Backend.Exists(*key) {
				log.Println("already exists, don't need to do anything")
				fmt.Fprintf(w, key.String())
				return
			}
			f.Seek(0, 0)
			err = s.Backend.Write(*key, f)
			if err != nil {
				http.Error(w, "could not write file", 500)
				return
			}
			fmt.Fprintf(w, key.String())
			return
		} else {
			fmt.Fprintf(w, "show form/handle post\n")
			return
		}
	}
	if len(parts) == 4 {
		key := parts[2]
		log.Printf("%s /local/%s/\n", r.Method, parts[2])
		k, err := KeyFromString(key)
		if err != nil {
			http.Error(w, "invalid key\n", 400)
			return
		}
		if !s.Backend.Exists(*k) {
			http.Error(w, "not found\n", 404)
			return
		}
		if r.Method == "HEAD" {
			w.WriteHeader(200)
			return
		}
		if r.Method == "GET" {
			data, err := s.Backend.Read(*k)
			if err != nil {
				log.Println(err)
				http.Error(w, "error reading file", 500)
				return
			}
			w.Header().Set("Content-Type", "application/octet")
			w.Write(data)
		}
	}
}

func serveDirect(w http.ResponseWriter, key Key, s *Site) bool {
	if !s.Backend.Exists(key) {
		return false
	}
	data, err := s.Backend.Read(key)
	if err != nil {
		log.Println(err)
		return false
	}
	w.Header().Set("Content-Type", "application/octet")
	w.Write(data)
	log.Println("served direct")
	return true
}

func fileHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	parts := strings.Split(r.URL.String(), "/")
	if len(parts) == 4 {
		key := parts[2]
		log.Printf("%s /file/%s/\n", r.Method, parts[2])
		k, err := KeyFromString(key)
		if err != nil {
			http.Error(w, "invalid key\n", 400)
			return
		}
		if serveDirect(w, *k, s) {
			return
		}
		data, err := s.Cluster.Retrieve(*k)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		w.Write(data)
	} else {
		http.Error(w, "bad request", 400)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	if r.Method == "GET" {
		clusterInfoHandler(w, r, s)
		return
	}
	if r.Method == "POST" {
		postFileHandler(w, r, s)
		return
	}
	http.Error(w, "method not supported", 405)
}

type clusterInfoPage struct {
	Title     string
	Cluster   *Cluster
	Myself    *Node
	Neighbors []Node
	Site      *Site
}

func clusterInfoHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	p := clusterInfoPage{
		Title:     "cluster status",
		Cluster:   s.Cluster,
		Myself:    s.Node,
		Neighbors: s.Cluster.NeighborsInclusive(),
		Site:      s,
	}
	t, _ := template.New("cluster").Parse(cluster_template)
	t.Execute(w, p)
}

var DEFAULT_REPLICATION = 3
var MIN_REPLICATION = 1

type postResponse struct {
	Key     string `json:"key"`
	Success bool   `json:"success"`
}

func postFileHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	log.Println("add a file")

	f, _, _ := r.FormFile("file")
	defer f.Close()
	h := sha1.New()
	io.Copy(h, f)
	key, err := KeyFromString("sha1:" + fmt.Sprintf("%x", h.Sum(nil)))
	if err != nil {
		log.Println(err)
		http.Error(w, "bad hash", 500)
		return
	}
	f.Seek(0, 0)
	success := s.Cluster.AddFile(*key, f, DEFAULT_REPLICATION, MIN_REPLICATION)
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

func joinHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	if r.Method == "POST" {
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
		n, err := s.Cluster.JoinNeighbor(u)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}
		fmt.Fprintf(w, fmt.Sprintf("Added node [%s]", n.UUID))
	} else {
		// show form
		w.Write([]byte(join_template))
	}
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		var hb heartbeat
		err := decoder.Decode(&hb)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if !s.Cluster.CheckSecret(hb.Secret) {
			log.Println("got an unauthorized heartbeat")
			http.Error(w, "sorry, need the secret knock", 403)
			return
		}
		n := Node{
			UUID: hb.UUID, BaseUrl: hb.BaseUrl, Writeable: hb.Writeable,
			LastSeen: time.Now()}
		s.Cluster.UpdateNeighbor(n)
		for _, neighbor := range hb.Neighbors {
			if neighbor.UUID == s.Node.UUID {
				// skip ourselves as usual
				continue
			}
			_, found := s.Cluster.FindNeighborByUUID(neighbor.UUID)
			if !found {
				log.Println("learned about a new neighbor via heartbeat")
				log.Println(neighbor.UUID, neighbor.BaseUrl)
				s.Cluster.JoinNeighbor(neighbor.BaseUrl)
			}
		}
	} else {
		http.Error(w, "method not allowed", 405)
	}
}

func configHandler(w http.ResponseWriter, r *http.Request, s *Site) {
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

const cluster_template = `
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
</head>
<body>
<div class="container">
<h1>Node: {{.Myself.UUID}}</h1>
<table class="table">
<tr><th>Base</th><td>{{.Myself.BaseUrl}}</td></tr>
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
<td><a href="{{.BaseUrl}}">{{.BaseUrl}}</a></td>
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

const join_template = `
<html><head><title>Add Node</title></head>
<body>
<h1>Add Node</h1>
<form action="." method="post">
<input type="text" name="url" placeholder="Base URL" size="128" /><br />
<input type="text" name="secret" placeholder="cluster secret" /><br />
<input type="submit" value="add node" />
</form>
</body>
</html>
`
