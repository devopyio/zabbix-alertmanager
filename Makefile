GIT_HASH := $(shell git rev-parse HEAD)
DATE := $(shell date +%FT%T%z)
USER := $(shell whoami)
BRANCH := $(shell git branch | grep \* | cut -d ' ' -f2)
GO111MODULE := on
all: go-deps go-build docker-build

go-deps:
	go mod download

go-build:
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -X main.version=$(GIT_HASH) -X main.date="$(DATE)" -X main.branch=$(BRANCH) -X main.revision=$(GIT_HASH) -X main.user=$(USER) -extldflags "-static"' ./cmd/zal/

docker-build:
	docker build . -t zabbix-alertmanager

docker-login:
	docker login -u $(DOCKER_USERNAME) -p $(DOCKER_PASSWORD)

docker-push:
	docker tag zabbix-alertmanager $(DOCKER_USERNAME)/zabbix-alertmanager:$(GIT_HASH)
	docker push $(DOCKER_USERNAME)/zabbix-alertmanager:$(GIT_HASH)
