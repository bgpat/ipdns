NAME     := ipdns
VERSION  := 1.0.0
REVISION := $(shell git rev-parse --short HEAD)
BUILT_AT := $(shell date +%s)
LDFLAGS  := -ldflags="-s -w -extldflags '-static' -X 'main.version=$(VERSION)' -X 'main.revision=$(REVISION)' -X 'main.builtAt=$(BUILT_AT)'"

bin/$(NAME): main.go vendor/*
	go build $(LDFLAGS) -o bin/$(NAME)

.PHONY: clean
clean:
	rm -rf bin/* vendor/*

.PHONY: dep
dep:
ifeq ($(shell command -v dep 2> /dev/null),)
	go get -u github.com/golang/dep/cmd/dep
endif

vendor/*: Gopkg.toml Gopkg.lock
	dep ensure -vendor-only
