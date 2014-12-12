run: cask .env
	./run.sh

cask: *.go
	go build .

install_deps:
	go get github.com/kelseyhightower/envconfig

cluster: cask .env .env2
	./run.sh&
	./run2.sh
