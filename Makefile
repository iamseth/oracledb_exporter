VERSION := 0.2.1
LDFLAGS := -X main.Version=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS) -s -w"
GOARCH ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m)))
ORA_RPM = oracle-instantclient18.3-devel-18.3.0.0.0-3.x86_64.rpm oracle-instantclient18.3-basic-18.3.0.0.0-3.x86_64.rpm
LD_LIBRARY_PATH = /usr/lib/oracle/18.3/client64/lib

%.rpm:
	wget http://yum.oracle.com/repo/OracleLinux/OL7/oracle/instantclient/x86_64/getPackage/$@

prereq: $(ORA_RPM)
	@echo deps
	@sudo apt-get -qq update
	@sudo apt-get install --no-install-recommends -qq libaio1 rpm
	@sudo rpm -Uvh --nodeps oracle*rpm
	@echo $(LD_LIBRARY_PATH) | sudo tee /etc/ld.so.conf.d/oracle.conf
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

docker: ubuntu-image alpine-image

ubuntu-image: $(ORA_RPM)
	docker build --build-arg VERSION=$(VERSION) -t "iamseth/oracledb_exporter:$(VERSION)" .
	docker tag "iamseth/oracledb_exporter:$(VERSION)" "iamseth/oracledb_exporter:latest"

alpine-image: $(ORA_RPM)
	docker build -f alpine/Dockerfile --build-arg VERSION=$(VERSION) -t "iamseth/oracledb_exporter:$(VERSION)" .
	docker tag "iamseth/oracledb_exporter:$(VERSION)" "iamseth/oracledb_exporter:latest"

travis: prereq deps test linux darwin docker
	@true

.PHONY: build deps test clean docker travis
