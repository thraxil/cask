run: cask .env
	/bin/bash .env && ./cask

cask: *.go
	go build .
