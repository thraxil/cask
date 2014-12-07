package main

import (
	"fmt"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request, s *Site) {
	fmt.Fprintf(w, "hello")
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// just ignore this crap
}
