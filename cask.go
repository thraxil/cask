package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, *Site), s *Site) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, s)
	}
}

type Config struct {
	Writeable         bool
	BaseUrl           string `envconfig:"BASE_URL"`
	UUID              string
	Backend           string
	DiskBackendRoot   string `envconfig:"DISK_BACKEND_ROOT"`
	S3AccessKey       string `envconfig:"S3_ACCESS_KEY"`
	S3SecretKey       string `envconfig:"S3_SECRET_KEY"`
	S3Bucket          string `envconfig:"S3_BUCKET"`
	Port              int
	Neighbors         string
	Replication       int
	MaxReplication    int    `envconfig:"MAX_REPLICATION"`
	ClusterSecret     string `envconfig:"CLUSTER_SECRET"`
	HeartbeatInterval int    `envconfig:"HEARTBEAT_INTERVAL"`
	AAEInterval       int    `envconfig:"AAE_INTERVAL"`
	MaxProcs          int    `envconfig:"MAX_PROCS"`
	SSL_Cert          string `envconfig:"SSL_CERT"`
	SSL_Key           string `envconfig:"SSL_Key"`
}

func main() {
	var c Config
	err := envconfig.Process("cask", &c)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.SetPrefix(c.UUID[:8] + " ")
	n := NewNode(c.UUID, c.BaseUrl, c.Writeable)
	var backend Backend
	if c.Backend == "disk" {
		backend = NewDiskBackend(c.DiskBackendRoot)
	} else if c.Backend == "s3" {
		if c.S3AccessKey == "" || c.S3SecretKey == "" || c.S3Bucket == "" {
			log.Fatal("need S3 ACCESS_KEY, SECRET_KEY, and bucket all configured")
		} else {
			backend = NewS3Backend(c.S3AccessKey, c.S3SecretKey, c.S3Bucket)
		}
	}
	if c.MaxProcs > 0 {
		log.Printf("max procs: %d\n", c.MaxProcs)
		runtime.GOMAXPROCS(c.MaxProcs)
	} else {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	cluster := NewCluster(*n, c.ClusterSecret, c.HeartbeatInterval)
	s := NewSite(n, cluster, backend, c.Replication, c.MaxReplication, c.ClusterSecret, c.AAEInterval)
	if c.Neighbors != "" {
		go cluster.BootstrapNeighbors(c.Neighbors)
	}
	go cluster.Heartbeat()
	go s.ActiveAntiEntropy()
	go cluster.Reaper()

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

	if c.SSL_Cert != "" && c.SSL_Key != "" && strings.HasPrefix(c.BaseUrl, "https:") {
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf(":%d", c.Port), c.SSL_Cert, c.SSL_Key, nil))
	} else {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", c.Port), nil))
	}
}
