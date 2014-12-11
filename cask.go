package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/kelseyhightower/envconfig"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, *Site), s *Site) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, s)
	}
}

type Config struct {
	Writeable       bool
	BaseUrl         string `envconfig:"BASE_URL"`
	UUID            string
	DiskBackendRoot string `envconfig:"DISK_BACKEND_ROOT"`
	Port            int
}

func main() {
	var c Config
	err := envconfig.Process("cask", &c)
	if err != nil {
		log.Fatal(err.Error())
	}

	n := NewNode(c.UUID, c.BaseUrl, c.Writeable)
	backend := NewDiskBackend(c.DiskBackendRoot)
	s := NewSite(n, backend)

	log.Println("=== Cask Node starting ================")
	log.Println("Root: " + c.DiskBackendRoot)
	log.Println("UUID: " + c.UUID)
	log.Println("Base URL: " + c.BaseUrl)
	log.Println("=======================================")

	http.HandleFunc("/", makeHandler(helloHandler, s))
	http.HandleFunc("/local/", makeHandler(localHandler, s))
	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", c.Port), nil))
}
