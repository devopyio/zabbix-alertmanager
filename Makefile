GIT_HASH := $(shell git rev-parse HEAD)

all: go-deps go-build docker-build
go-deps:
	go mod download
go-build:
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X main.Version=$(GIT_HASH) -extldflags "-static"' ./cmd/zal/
docker-build:
	docker build . -t alertmanager-zabbix-webhook
