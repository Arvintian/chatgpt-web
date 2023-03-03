GIT_VERSION = $(shell git rev-parse --short HEAD)

.PHONY: build
build:
	mkdir -p dist && docker run --rm -ti -e GOPROXY=https://goproxy.cn,direct -v `pwd`:/app -w /app golang:1.19-alpine \
	go build -v --ldflags="-w -X main.Version=$(GIT_VERSION)" -o dist/server cmd/*.go

clean:
	rm -rf dist