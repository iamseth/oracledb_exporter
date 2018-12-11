VERSION := 0.2.0
LDFLAGS := -X main.Version=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS) -s -w"
GOARCH ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m)))


oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm:
	wget -q https://www.dropbox.com/s/f2ul3y0854y8oqw/oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm -O oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm

oracle-instantclient12.2-devel-12.2.0.1.0-1.x86_64.rpm:
	wget -q https://www.dropbox.com/s/qftd81ezcp8k9kd/oracle-instantclient12.2-devel-12.2.0.1.0-1.x86_64.rpm -O oracle-instantclient12.2-devel-12.2.0.1.0-1.x86_64.rpm

prereq: oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm oracle-instantclient12.2-devel-12.2.0.1.0-1.x86_64.rpm
	@echo deps
	@sudo apt-get -qq update
	@sudo apt-get install --no-install-recommends -qq libaio1 rpm
	@sudo rpm -Uvh --nodeps oracle*rpm
	@echo /usr/lib/oracle/12.2/client64/lib | sudo tee /etc/ld.so.conf.d/oracle.conf
	@sudo ldconfig

linux:
	@echo build linux
	@mkdir -p ./dist/oracledb_exporter.$(VERSION).linux-${GOARCH}
	@PKG_CONFIG_PATH=${PWD} GOOS=linux go build $(GOFLAGS) -o ./dist/oracledb_exporter.$(VERSION).linux-${GOARCH}/oracledb_exporter
	@cp default-metrics.toml ./dist/oracledb_exporter.$(VERSION).linux-${GOARCH}
	@(cd dist ; tar cfz oracledb_exporter.$(VERSION).linux-${GOARCH}.tar.gz oracledb_exporter.$(VERSION).linux-${GOARCH})

darwin:
	@echo build darwin
	@mkdir -p ./dist/oracledb_exporter.$(VERSION).darwin-${GOARCH}
	@PKG_CONFIG_PATH=${PWD} GOOS=darwin go build $(GOFLAGS) -o ./dist/oracledb_exporter.$(VERSION).darwin-${GOARCH}/oracledb_exporter
	@cp default-metrics.toml ./dist/oracledb_exporter.$(VERSION).darwin-${GOARCH}
	@(cd dist ; tar cfz oracledb_exporter.$(VERSION).darwin-${GOARCH}.tar.gz oracledb_exporter.$(VERSION).darwin-${GOARCH})

local-build:  linux

build: docker

deps:
	@PKG_CONFIG_PATH=${PWD} go get

test:
	@echo test
	@PKG_CONFIG_PATH=${PWD} go test $$(go list ./... | grep -v /vendor/)

clean:
	@rm -rf ./dist

docker: oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm oracle-instantclient12.2-devel-12.2.0.1.0-1.x86_64.rpm
	docker build --build-arg VERSION=$(VERSION) -t "yannig/oracledb_exporter:${VERSION}" .
	docker tag yannig/oracledb_exporter:${VERSION} yannig/oracledb_exporter:latest

travis: prereq deps test linux darwin
	@true

.PHONY: build deps test clean docker travis
