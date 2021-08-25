GOCMD=go
GOBUILD=$(GOCMD) build

BINARY_NAME=pi-web
BINARY_NAME_LINUX_AMD64=pi-web-linux-amd64
BINARY_NAME_LINUX_ARM=pi-web-linux-arm
BINARY_NAME_OPENBSD_AMD64=pi-web-openbsd-amd64

GIT_COMMIT := $(shell git rev-parse HEAD)

build:
	$(GOBUILD) -o $(BINARY_NAME) -ldflags="-X github.com/aaronriekenberg/pi-web/environment.gitCommit=$(GIT_COMMIT)"

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME_LINUX_AMD64) -ldflags="-X github.com/aaronriekenberg/pi-web/environment.gitCommit=$(GIT_COMMIT)"

build-linux-arm:
	GOOS=linux GOARCH=arm $(GOBUILD) -o $(BINARY_NAME_LINUX_ARM) -ldflags="-X github.com/aaronriekenberg/pi-web/environment.gitCommit=$(GIT_COMMIT)"

build-openbsd-amd64:
	GOOS=openbsd GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME_OPENBSD_AMD64) -ldflags="-X github.com/aaronriekenberg/pi-web/environment.gitCommit=$(GIT_COMMIT)"
