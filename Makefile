cask: *.go
	go build .

.PHONY: test
test: cask
	go test .

install_deps:
	go get github.com/kelseyhightower/envconfig
	go get github.com/mitchellh/goamz/aws
	go get github.com/mitchellh/goamz/s3
	go get golang.org/x/oauth2
	go get github.com/prometheus/client_golang/prometheus
	go get github.com/prometheus/procfs
	go get github.com/hashicorp/memberlist

clustertmp:
	mkdir /tmp/cask{0,1,2,3,4,5,6}

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
