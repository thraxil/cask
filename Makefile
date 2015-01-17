cask: *.go
	go build .

test: cask
	go test .

install_deps:
	go get github.com/kelseyhightower/envconfig
	go get github.com/mitchellh/goamz/aws
	go get github.com/mitchellh/goamz/s3

cluster: cask
	python run_cluster.py

install: cask
	cp cask /usr/local/bin/
