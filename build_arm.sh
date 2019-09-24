#!/bin/bash

export GIT_COMMIT=$(git rev-parse HEAD)
GOOS=linux GOARCH=arm go build -x -ldflags "-X main.gitCommit=$GIT_COMMIT"
