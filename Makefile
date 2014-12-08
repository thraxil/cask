run: cask .env
	./run.sh

cask: *.go
	go build .
