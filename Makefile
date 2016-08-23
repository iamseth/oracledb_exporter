VERSION := 0.0.1

LDFLAGS := -X main.Version=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS) -s -w"
GOOS ?= $(shell uname | tr A-Z a-z)
GOARCH ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m)))
SUFFIX ?= $(GOOS)-$(GOARCH)
ARCHIVE ?= $(BINARY)-$(VERSION).$(SUFFIX).tar.gz
BINARY := oracledb_exporter-$(VERSION).$(SUFFIX)

./dist/$(BINARY):
	mkdir -p ./dist
	go build $(GOFLAGS) -o $@

.PHONY: test
test:
	go test $$(go list ./... | grep -v /vendor/)

.PHONY: clean
clean:
	rm -rf ./dist

.PHONY: docker
docker:
	docker run --rm -v "$$PWD":/go/src/github.com/iamseth/oracledb_exporter -w /go/src/github.com/iamseth/oracledb_exporter golang:1.7 bash -c make
