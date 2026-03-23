.PHONY: all help frontend-build build build-cli test docker-build docker-run docker-restart clean

# Defaults (override on command line):
REGISTRY ?= ghcr.io/example
IMAGE_NAME ?= agent-forum
IMAGE_VERSION ?= $(shell date +%Y%m%d_%H%M%S)
IMAGE ?= $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_VERSION)
LATEST_TAG ?= latest
IMAGE_LATEST ?= $(REGISTRY)/$(IMAGE_NAME):$(LATEST_TAG)

GO ?= go
NPM ?= npm

help:
	@echo "Usage: make [target] [REGISTRY=...] [IMAGE_NAME=...] [IMAGE_VERSION=...] [LATEST_TAG=...]"
	@echo "Targets: build, docker-build, docker-run, docker-restart, test, clean, help"

frontend-build:
	cd frontend && $(NPM) install --no-audit --no-fund && $(NPM) run build
	rm -rf internal/web/static/*
	cp -a frontend/dist/. internal/web/static/

build: frontend-build
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build -o bin/forum-server ./cmd/server

build-cli:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o bin/forumctl ./cmd/cli

test:
	$(GO) test -v -coverprofile=coverage.out ./...

# Build docker image; set REGISTRY, IMAGE_NAME, IMAGE_VERSION to control the image tag
docker-build: build
	@echo "Building image: $(IMAGE)"
	@echo "Also tagging as: $(IMAGE_LATEST)"
	docker build -t $(IMAGE) -t $(IMAGE_LATEST) .
	docker push $(IMAGE)
	docker push $(IMAGE_LATEST)

# Run using the latest tag by default
docker-run:
	-@docker rm -f agent-forum 2>/dev/null || true
	docker run -d --name agent-forum --restart=unless-stopped -p 8080:8080 -v $(PWD)/forum.db:/data/forum.db $(IMAGE_LATEST)

docker-restart:
	-@docker rm -f agent-forum 2>/dev/null || true
	docker run -d --name agent-forum --restart=unless-stopped -p 8080:8080 -v $(PWD)/forum.db:/data/forum.db $(IMAGE_LATEST)
	docker ps -a | grep agent-forum || true
	curl -s http://localhost:8080/health || true

clean:
	rm -rf bin/ coverage.out frontend/dist
