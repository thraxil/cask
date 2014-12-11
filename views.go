package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"text/template"
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
