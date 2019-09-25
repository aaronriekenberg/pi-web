GOCMD=go
GOBUILD=$(GOCMD) build

BINARY_NAME=pi-web
BINARY_NAME_LINUX_AMD64=pi-web-linux-amd64
BINARY_NAME_LINUX_ARM=pi-web-linux-arm

GIT_COMMIT := $(shell git rev-parse HEAD)

all: build build-linux-amd64 build-linux-arm

build:
	$(GOBUILD) -o $(BINARY_NAME) -ldflags="-X main.gitCommit=$(GIT_COMMIT)"

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME_LINUX_AMD64) -ldflags="-X main.gitCommit=$(GIT_COMMIT)"

build-linux-arm:
	GOOS=linux GOARCH=arm $(GOBUILD) -o $(BINARY_NAME_LINUX_ARM) -ldflags="-X main.gitCommit=$(GIT_COMMIT)"
