VERSION := 0.0.7

LDFLAGS := -X main.Version=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS) -s -w"
GOARCH ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m)))


build:
	@echo build
	@mkdir -p ./dist
	@PKG_CONFIG_PATH=${PWD} GOOS=linux go build $(GOFLAGS) -o ./dist/oracledb_exporter.linux-${GOARCH}
	@PKG_CONFIG_PATH=${PWD} GOOS=darwin go build $(GOFLAGS) -o ./dist/oracledb_exporter.darwin-${GOARCH}

deps:
	@echo deps
	@sudo apt-get -qq update
	@sudo apt-get install --no-install-recommends -qq libaio1 rpm
	@wget https://www.dropbox.com/s/f2ul3y0854y8oqw/oracle-instantclient12.2-basic-12.2.0.1.0-1.x86_64.rpm
	@wget https://www.dropbox.com/s/qftd81ezcp8k9kd/oracle-instantclient12.2-devel-12.2.0.1.0-1.x86_64.rpm
	@sudo rpm -Uvh --nodeps oracle*rpm
	@echo /usr/lib/oracle/12.2/client64/lib | sudo tee /etc/ld.so.conf.d/oracle.conf
	@sudo ldconfig

test:
	@echo test
	@PKG_CONFIG_PATH=${PWD} go test $$(go list ./... | grep -v /vendor/)

clean:
	@rm -rf ./dist

docker:
	@docker build -t "iamseth/oracledb_exporter:${VERSION}" .
	@docker tag iamseth/oracledb_exporter:${VERSION} iamseth/oracledb_exporter:latest

travis: deps test build docker
	@true

.PHONY: build deps test clean docker travis
