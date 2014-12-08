package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func helloHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	fmt.Fprintf(w, "hello")
}

func localHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	parts := strings.Split(r.URL.String(), "/")
	fmt.Fprintf(w, fmt.Sprintf("%d %s\n", len(parts), parts))
	if len(parts) == 3 {
		if r.Method == "POST" {
			fmt.Fprintf(w, "write a file")

			f, _, _ := r.FormFile("file")
			defer f.Close()
			h := sha1.New()
			io.Copy(h, f)
			key, err := KeyFromString("sha1:" + fmt.Sprintf("%x", h.Sum(nil)))
			if err != nil {
				http.Error(w, "bad hash", 500)
				return
			}
			f.Seek(0, 0)
			s.Backend.Write(*key, f)
			return key.String()
		} else {
			fmt.Fprintf(w, "show form/handle post\n")
			return
		}
	}
	if len(parts) == 4 {
		key := parts[2]
		fmt.Fprintf(w, fmt.Sprintf("retrieve file with key %s\n", parts[2]))
		k, err := KeyFromString(key)
		if err != nil {
			fmt.Fprintf(w, "invalid key\n")
			return
		}
		fmt.Fprintf(w, k.String()+"\n")
	}
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// just ignore this crap
}
