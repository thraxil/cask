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
	Neighbors       string
	Replication     int
	MaxReplication  int `envconfig:"MAX_REPLICATION"`
}

func main() {
	var c Config
	err := envconfig.Process("cask", &c)
	if err != nil {
		log.Fatal(err.Error())
	}

	n := NewNode(c.UUID, c.BaseUrl, c.Writeable)
	backend := NewDiskBackend(c.DiskBackendRoot)
	cluster := NewCluster(*n)
	s := NewSite(n, cluster, backend, c.Replication, c.MaxReplication)
	if c.Neighbors != "" {
		go cluster.BootstrapNeighbors(c.Neighbors)
	}
	go cluster.Heartbeat()
	go s.ActiveAntiEntropy()

	log.Println("=== Cask Node starting ================")
	log.Println("Root: " + c.DiskBackendRoot)
	log.Println("UUID: " + c.UUID)
	log.Println("Base URL: " + c.BaseUrl)
	log.Println("=======================================")

	http.HandleFunc("/", makeHandler(indexHandler, s))
	http.HandleFunc("/local/", makeHandler(localHandler, s))
	http.HandleFunc("/file/", makeHandler(fileHandler, s))
	http.HandleFunc("/join/", makeHandler(joinHandler, s))
	http.HandleFunc("/config/", makeHandler(configHandler, s))
	http.HandleFunc("/heartbeat/", makeHandler(heartbeatHandler, s))

	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", c.Port), nil))
}
