package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// just ignore this crap
}
