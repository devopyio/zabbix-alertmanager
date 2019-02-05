GIT_HASH := $(shell git rev-parse HEAD)

all: go-deps go-build docker-build docker-login docker-push

go-deps:
	go mod download

go-build:
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X main.Version=$(GIT_HASH) -extldflags "-static"' ./cmd/zal/

docker-build:
	docker build . -t zabbix-alertmanager

docker-login:
	docker login -u $(DOCKER_USERNAME) -p $(DOCKER_PASSWORD)

docker-push:
	docker tag zabbix-alertmanager $(DOCKER_USERNAME)/zabbix-alertmanager:$(GIT_HASH)
	docker push $(DOCKER_USERNAME)/zabbix-alertmanager:$(GIT_HASH)
