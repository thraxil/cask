cask: *.go go.mod go.sum
	go build .

.PHONY: test
test: cask
	go test .

clustertmp:
	mkdir -p /tmp/cask{0,1,2,3,4,5,6}

.PHONY: cluster
cluster: cask clustertmp
	python run_cluster.py
