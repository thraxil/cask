package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello")
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// just ignore this crap
}

func main() {
	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/favicon.ico", faviconHandler)
	log.Fatal(http.ListenAndServe(":"+os.Getenv("CASK_PORT"), nil))
}
