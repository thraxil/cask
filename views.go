package main

import (
	"fmt"
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
			return
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
