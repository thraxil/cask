cask: *.go glide.*
	go build .

test: cask
	go test .

install_deps:
	go get github.com/kelseyhightower/envconfig
	go get github.com/mitchellh/goamz/aws
	go get github.com/mitchellh/goamz/s3
	go get golang.org/x/oauth2
	go get github.com/stacktic/dropbox

cluster: cask
	python run_cluster.py

install: cask
	cp cask /usr/local/bin/
