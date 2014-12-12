package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"
)

func helloHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	fmt.Fprintf(w, "hello")
}

// read/write file requests that shall only touch
// the current node. No cluster interaction.
func localHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	parts := strings.Split(r.URL.String(), "/")
	log.Printf("%d %s\n", len(parts), parts)
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
			s.Backend.Write(*key, f)
			fmt.Fprintf(w, key.String())
			return
		} else {
			fmt.Fprintf(w, "show form/handle post\n")
			return
		}
	}
	if len(parts) == 4 {
		key := parts[2]
		log.Printf("retrieve file with key %s\n", parts[2])
		k, err := KeyFromString(key)
		if err != nil {
			http.Error(w, "invalid key\n", 400)
			return
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
		w.Write(data)
	}
}

type clusterInfoPage struct {
	Title     string
	Cluster   *Cluster
	Neighbors []Node
}

func clusterInfoHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	p := clusterInfoPage{
		Title:     "cluster status",
		Cluster:   s.Cluster,
		Neighbors: s.Cluster.NeighborsInclusive(),
	}
	t, _ := template.New("cluster").Parse(cluster_template)
	t.Execute(w, p)
}

func joinHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	if r.Method == "POST" {
		if r.FormValue("url") == "" {
			fmt.Fprint(w, "no url specified")
			return
		}
		u := r.FormValue("url")
		config_url := u + "/config/"
		res, err := http.Get(config_url)
		if err != nil {
			fmt.Fprint(w, "error retrieving config")
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Fprintf(w, "error reading body of response")
			return
		}
		var n Node
		err = json.Unmarshal(body, &n)
		if err != nil {
			fmt.Fprintf(w, "error parsing json")
			return
		}

		if n.UUID == s.Node.UUID {
			fmt.Fprintf(w, "I can't join myself, silly!")
			return
		}
		_, ok := s.Cluster.FindNeighborByUUID(n.UUID)
		if ok {
			fmt.Fprintf(w, "already have a node with that UUID in the cluster")
			// let's not do updates through this. Let gossip handle that.
			return
		}
		n.LastSeen = time.Now()
		s.Cluster.AddNeighbor(n)
		// join the node to all our neighbors too
		for _, neighbor := range s.Cluster.GetNeighbors() {
			if neighbor.UUID == n.UUID {
				// obviously, skip the one we just added
				continue
			}
			res, err = http.PostForm(neighbor.BaseUrl+"/join/",
				url.Values{"url": {u}})
			if err != nil {
				fmt.Println(err)
			} else {
				res.Body.Close()
			}

		}
		// reciprocate
		res, err = http.PostForm(n.BaseUrl+"/join/",
			url.Values{"url": {s.Node.BaseUrl}})
		if err != nil {
			fmt.Println(err)
			return
		}
		res.Body.Close()

		fmt.Fprintf(w, fmt.Sprintf("Added node [%s]", n.UUID))

	} else {
		// show form
		w.Write([]byte(join_template))
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
</head>
<body>
<table border="1">
<tr>
<th>UUID</th>
<th>Base</th>
<th>Writeable</th>
<th>LastSeen</th>
<th>LastFailed</th>
</tr>
{{range .Neighbors}}
<tr>
<td>{{.UUID}}</td>
<td>{{.BaseUrl}}</td>
<td>{{.Writeable}}</td>
<td>{{.LastSeen}}</td>
<td>{{.LastFailed}}</td>
</tr>
{{end}}
</table>
</body>
</html>
`

const join_template = `
<html><head><title>Add Node</title></head>
<body>
<h1>Add Node</h1>
<form action="." method="post">
<input type="text" name="url" placeholder="Base URL" size="128" /><br />
<input type="submit" value="add node" />
</form>
</body>
</html>
`
