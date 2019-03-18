DATE := $(shell date +%FT%T%z)
USER := $(shell whoami)
GIT_HASH := $(shell git --no-pager describe --tags --always)
BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GO111MODULE := on
all: go-deps go-test go-build docker-push

go-deps:
	go mod download

go-build:
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -X github.com/prometheus/common/version.Version=$(GIT_HASH) -X github.com/prometheus/common/version.BuildDate="$(DATE)" -X github.com/prometheus/common/version.Branch=$(BRANCH) -X github.com/prometheus/common/version.Revision=$(GIT_HASH) -X github.com/prometheus/common/version.BuildUser=$(USER) -extldflags "-static"' ./cmd/zal/

.PHONY: go-test
go-test:
	$(BUILDENV) go test $(TESTFLAGS) ./...

docker-build:
	docker build . -t zabbix-alertmanager

docker-login:
	docker login -u $(DOCKER_USERNAME) -p $(DOCKER_PASSWORD)

docker-push: docker-build docker-login
	docker tag zabbix-alertmanager $(DOCKER_USERNAME)/zabbix-alertmanager:$(GIT_HASH)
	docker push $(DOCKER_USERNAME)/zabbix-alertmanager:$(GIT_HASH)
