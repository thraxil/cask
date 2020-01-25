GOBIN ?= go

cask: *.go
	$(GOBIN) build .

test: cask
	$(GOBIN) test .

install_deps:
	$(GOBIN) get github.com/kelseyhightower/envconfig
	$(GOBIN) get github.com/mitchellh/goamz/aws
	$(GOBIN) get github.com/mitchellh/goamz/s3
	$(GOBIN) get golang.org/x/oauth2
	$(GOBIN) get github.com/prometheus/client_golang/prometheus
	$(GOBIN) get github.com/prometheus/procfs
	$(GOBIN) get github.com/hashicorp/memberlist
	$(GOBIN) get -u github.com/honeycombio/beeline-go/...

cluster: cask
	python run_cluster.py

install: cask
	cp cask /usr/local/bin/cask1
	mv /usr/local/bin/cask1 /usr/local/bin/cask

restart_all:
	sudo systemctl stop cask-sata1 && sudo systemctl start cask-sata1
	sudo systemctl stop cask-sata2 && sudo systemctl start cask-sata2
	sudo systemctl stop cask-sata3 && sudo systemctl start cask-sata3
	sudo systemctl stop cask-sata4 && sudo systemctl start cask-sata4
	sudo systemctl stop cask-sata5 && sudo systemctl start cask-sata5
	sudo systemctl stop cask-sata7 && sudo systemctl start cask-sata7
	sudo systemctl stop cask-sata8 && sudo systemctl start cask-sata8
	sudo systemctl stop cask-sata9 && sudo systemctl start cask-sata9
	sudo systemctl stop cask-sata10 && sudo systemctl start cask-sata10
	sudo systemctl stop cask-sata11 && sudo systemctl start cask-sata11
	sudo systemctl stop cask-sata12 && sudo systemctl start cask-sata12
