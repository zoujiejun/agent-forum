.PHONY: frontend-build build build-cli test docker-build docker-run docker-restart clean

REGISTRY ?= ghcr.io/example
IMAGE_NAME ?= agent-forum
IMAGE_REPOSITORY ?= $(REGISTRY)/$(IMAGE_NAME)
IMAGE_MASTER ?= $(IMAGE_REPOSITORY):master
GO ?= go
NPM ?= npm
BUILD_DATE := $(shell date +%Y%m%d)
BUILD_TIME := $(shell date +%H%M%S)
VERSION := $(BUILD_DATE)_$(BUILD_TIME)
IMAGE_VERSION ?= $(IMAGE_REPOSITORY):$(VERSION)

frontend-build:
	cd frontend && $(NPM) install && $(NPM) run build

build: frontend-build
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build -o bin/forum-server ./cmd/server

build-cli:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o bin/forumctl ./cmd/cli

test:
	$(GO) test -v -coverprofile=coverage.out ./...

docker-build: build
	@echo "Building image: $(IMAGE_MASTER)"
	@echo "Building image: $(IMAGE_VERSION)"
	sg docker -c 'docker build -t $(IMAGE_MASTER) -t $(IMAGE_VERSION) . && docker push $(IMAGE_MASTER) && docker push $(IMAGE_VERSION)'

docker-run:
	sg docker -c 'docker run -d --name agent-forum --restart=unless-stopped -p 8080:8080 -v $(PWD)/forum.db:/data/forum.db $(IMAGE_MASTER)'

docker-restart:
	sg docker -c 'docker rm -f agent-forum 2>/dev/null || true'
	sg docker -c 'docker run -d --name agent-forum --restart=unless-stopped -p 8080:8080 -v $(PWD)/forum.db:/data/forum.db $(IMAGE_MASTER)'
	sg docker -c 'docker ps -a | grep agent-forum'
	curl -s http://localhost:8080/health

clean:
	rm -rf bin/ coverage.out frontend/dist
