package main

import (
	_ "expvar"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, *site), s *site) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, s)
	}
}

type config struct {
	Writeable       bool
	BaseURL         string `envconfig:"BASE_URL"`
	UUID            string
	Backend         string
	DiskBackendRoot string `envconfig:"DISK_BACKEND_ROOT"`

	S3AccessKey string `envconfig:"S3_ACCESS_KEY"`
	S3SecretKey string `envconfig:"S3_SECRET_KEY"`
	S3Bucket    string `envconfig:"S3_BUCKET"`

	DBAccessKey string `envconfig:"DROPBOX_ACCESS_KEY"`
	DBSecretKey string `envconfig:"DROPBOX_SECRET_KEY"`
	DBToken     string `envconfig:"DROPBOX_TOKEN"`

	Port              int
	Neighbors         string
	Replication       int
	MaxReplication    int    `envconfig:"MAX_REPLICATION"`
	ClusterSecret     string `envconfig:"CLUSTER_SECRET"`
	HeartbeatInterval int    `envconfig:"HEARTBEAT_INTERVAL"`
	AAEInterval       int    `envconfig:"AAE_INTERVAL"`
	MaxProcs          int    `envconfig:"MAX_PROCS"`
	SSLCert           string `envconfig:"SSL_CERT"`
	SSLKey            string `envconfig:"SSL_Key"`
	ReadTimeout       int    `envconfig:"READ_TIMEOUT"`
	WriteTimeout      int    `envconfig:"WRITE_TIMEOUT"`
}

func main() {
	var c config
	err := envconfig.Process("cask", &c)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.SetPrefix(c.UUID[:8] + " ")
	n := newNode(c.UUID, c.BaseURL, c.Writeable)
	var backend backend
	if c.Backend == "disk" {
		backend = newDiskBackend(c.DiskBackendRoot)
	} else if c.Backend == "s3" {
		if c.S3AccessKey == "" || c.S3SecretKey == "" || c.S3Bucket == "" {
			log.Fatal("need S3 ACCESS_KEY, SECRET_KEY, and bucket all configured")
		} else {
			backend = newS3Backend(c.S3AccessKey, c.S3SecretKey, c.S3Bucket)
		}
	} else if c.Backend == "dropbox" {
		if c.DBAccessKey == "" || c.DBSecretKey == "" {
			log.Fatal("need dropbox ACCESS_KEY and SECRET_KEY")
		} else {
			backend = newDropboxBackend(c.DBAccessKey, c.DBSecretKey, c.DBToken)
		}
	}
	if c.MaxProcs > 0 {
		log.Printf("max procs: %d\n", c.MaxProcs)
		runtime.GOMAXPROCS(c.MaxProcs)
	} else {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	cluster := newCluster(*n, c.ClusterSecret, c.HeartbeatInterval)
	s := newSite(n, cluster, backend, c.Replication, c.MaxReplication, c.ClusterSecret, c.AAEInterval)
	if c.Neighbors != "" {
		go cluster.BootstrapNeighbors(c.Neighbors)
	}
	go cluster.Heartbeat()
	go s.ActiveAntiEntropy()
	go cluster.Reaper()

	log.Println("=== Cask Node starting ================")
	log.Println("Root: " + c.DiskBackendRoot)
	log.Println("UUID: " + c.UUID)
	log.Println("Base URL: " + c.BaseURL)
	log.Println("=======================================")

	http.HandleFunc("/", makeHandler(indexHandler, s))
	http.HandleFunc("/local/", makeHandler(localHandler, s))
	http.HandleFunc("/file/", makeHandler(fileHandler, s))
	http.HandleFunc("/join/", makeHandler(joinHandler, s))
	http.HandleFunc("/config/", makeHandler(configHandler, s))
	http.HandleFunc("/heartbeat/", makeHandler(heartbeatHandler, s))

	http.HandleFunc("/favicon.ico", faviconHandler)

	// some defaults
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 5
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 20
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", c.Port),
		ReadTimeout:  time.Duration(c.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(c.WriteTimeout) * time.Second,
	}
	if c.SSLCert != "" && c.SSLKey != "" && strings.HasPrefix(c.BaseURL, "https:") {
		log.Fatal(server.ListenAndServeTLS(c.SSLCert, c.SSLKey))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}
