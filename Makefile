cask: *.go go.mod go.sum
	go build .

.PHONY: test
test: cask
	go test .

.PHONY: coverage
coverage: cask
	go test -coverprofile=coverage.out ./...

.PHONY: coverage-report
coverage-report: coverage
	go tool cover -html=coverage.out

clustertmp:
	mkdir -p /tmp/cask{0,1,2,3,4,5,6}

.PHONY: cluster
cluster: cask clustertmp
	python run_cluster.py
