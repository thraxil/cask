package main

import (
	"log"
	"net/http"
	"os"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, *Site), s *Site) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, s)
	}
}

func main() {
	// get config from environment
	w := os.Getenv("CASK_WRITEABLE") == "True"
	uuid := os.Getenv("CASK_UUID")
	base_url := os.Getenv("CASK_BASE_URL")
	root := os.Getenv("CASK_DISK_BACKEND_ROOT")

	n := NewNode(uuid, base_url, w)
	backend := NewDiskBackend(root)
	s := NewSite(n, backend)

	log.Println("=== Cask Node starting ================")
	log.Println("Root: " + root)
	log.Println("UUID: " + uuid)
	log.Println("Base URL: " + base_url)
	log.Println("=======================================")

	http.HandleFunc("/", makeHandler(helloHandler, s))
	http.HandleFunc("/local/", makeHandler(localHandler, s))
	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("CASK_PORT"), nil))
}
