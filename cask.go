package main

import (
	_ "expvar"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, *site), s *site) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, s)
	}
}

var (
	broadcasts *memberlist.TransmitLimitedQueue
	mlist      *memberlist.Memberlist
)

var (
	// rebalance related
	rebalances = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cask_rebalance_total",
			Help: "The total number of rebalance attempts",
		})
	rebalanceFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cask_rebalance_failure_total",
		Help: "Keys that could not be rebalanced",
	})
	rebalanceNoops = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cask_rebalance_noop_total",
		Help: "Keys that do not need to be rebalanced",
	})
	rebalanceDeletes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cask_rebalance_delete_total",
		Help: "Keys that were removed from the local node",
	})
	// cluster related
	clusterJoins = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cask_cluster_joins_total",
		Help: "cluster join operations",
	})
	clusterLeaves = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cask_cluster_leaves_total",
		Help: "cluster leave operations",
	})
	clusterTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cask_cluster_total",
		Help: "total size of cluster",
	})
	// disk space
	diskFreeSpace = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cask_disk_free_bytes",
		Help: "free disk space available to this node",
	})
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(rebalances)
	prometheus.MustRegister(rebalanceFailures)
	prometheus.MustRegister(rebalanceNoops)
	prometheus.MustRegister(rebalanceDeletes)

	prometheus.MustRegister(clusterJoins)
	prometheus.MustRegister(clusterLeaves)
	prometheus.MustRegister(clusterTotal)

	prometheus.MustRegister(diskFreeSpace)
}

type config struct {
	Writeable       bool
	BaseURL         string `envconfig:"BASE_URL"`
	UUID            string
	Backend         string
	DiskBackendRoot string `envconfig:"DISK_BACKEND_ROOT"`
	KeepFree        uint64 `envconfig:"KEEP_FREE"`

	S3AccessKey string `envconfig:"S3_ACCESS_KEY"`
	S3SecretKey string `envconfig:"S3_SECRET_KEY"`
	S3Bucket    string `envconfig:"S3_BUCKET"`

	Port              int
	GossipPort        int `envconfig:"GOSSIP_PORT"`
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

	backend := setupBackend(c)

	if c.MaxProcs > 0 {
		log.Printf("max procs: %d\n", c.MaxProcs)
		runtime.GOMAXPROCS(c.MaxProcs)
	} else {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	if c.KeepFree == 0 {
		// default to keeping 10GB free
		c.KeepFree = 10 * 1024 * 1024 * 1024
	}
	cluster := newCluster(n, c.ClusterSecret, c.HeartbeatInterval)
	err = startMemberList(cluster, c)
	if err != nil {
		log.Fatal("couldn't start gossip", err)
	}
	s := newSite(n, cluster, backend, c.Replication, c.MaxReplication, c.ClusterSecret, c.AAEInterval)
	go s.ActiveAntiEntropy()
	go n.WatchFreeSpace(c.KeepFree, backend)

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

	http.HandleFunc("/favicon.ico", faviconHandler)
	http.Handle("/metrics", promhttp.Handler())

	// some defaults
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 5
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 20
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", c.Port),
		ReadTimeout:  time.Duration(c.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(c.WriteTimeout) * time.Second,
	}
	if c.SSLCert != "" && c.SSLKey != "" && strings.HasPrefix(c.BaseURL, "https:") {
		log.Fatal(server.ListenAndServeTLS(c.SSLCert, c.SSLKey))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}

func setupBackend(c config) backend {
	var backend backend
	if c.Backend == "disk" {
		backend = newDiskBackend(c.DiskBackendRoot)
	} else if c.Backend == "s3" {
		if c.S3AccessKey == "" || c.S3SecretKey == "" || c.S3Bucket == "" {
			log.Fatal("need S3 ACCESS_KEY, SECRET_KEY, and bucket all configured")
		} else {
			backend = newS3Backend(c.S3AccessKey, c.S3SecretKey, c.S3Bucket)
		}
	}
	return backend
}

func startMemberList(cluster *cluster, conf config) error {
	hostname, _ := os.Hostname()
	c := memberlist.DefaultLocalConfig()
	c.BindPort = conf.GossipPort
	c.Name = hostname + "-" + fmt.Sprintf("%d", conf.GossipPort)
	c.Delegate = cluster
	c.Events = cluster
	mlist, err := memberlist.Create(c)
	if err != nil {
		return err
	}
	if len(conf.Neighbors) > 0 {
		parts := strings.Split(conf.Neighbors, ",")
		_, err := mlist.Join(parts)
		if err != nil {
			log.Println(err)
		}
	}
	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return mlist.NumMembers()
		},
		RetransmitMult: 3,
	}

	return nil
}
