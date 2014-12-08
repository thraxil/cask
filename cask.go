package main

import (
	"fmt"
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
	w := os.Getenv("CASK_WRITEABLE") == "True"
	n := NewNode(os.Getenv("CASK_UUID"), os.Getenv("CASK_BASE_URL"), w)
	fmt.Println(os.Getenv("CASK_DISK_BACKEND_ROOT"))
	fmt.Println(os.Getenv("CASK_UUID"))
	backend := NewDiskBackend(os.Getenv("CASK_DISK_BACKEND_ROOT"))
	s := NewSite(n, backend)
	http.HandleFunc("/", makeHandler(helloHandler, s))
	http.HandleFunc("/local/", makeHandler(localHandler, s))

	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Fatal(http.ListenAndServe(":"+os.Getenv("CASK_PORT"), nil))
}
