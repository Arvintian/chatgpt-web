REGISTRY = arvintian
PROJECT = chatgpt-web
GIT_VERSION = $(shell git rev-parse --short HEAD)

.PHONY: build
build:
	mkdir -p dist && docker run --rm -ti -e GOPROXY=https://goproxy.cn,direct -v `pwd`:/app -w /app golang:1.19-alpine \
	go build -v --ldflags="-w -X main.Version=$(GIT_VERSION)" -o dist/server cmd/*.go

package: build
	docker build -t $(REGISTRY)/$(PROJECT):$(GIT_VERSION) .

clean:
	rm -rf dist
	docker images | grep -E "$(REGISTRY)/$(PROJECT)" | awk '{print $$3}' | uniq | xargs -I {} docker rmi --force {}