cask: *.go
	go build .

test: cask
	go test .

install_deps:
	go get github.com/kelseyhightower/envconfig

cluster: cask
	python run_cluster.py
