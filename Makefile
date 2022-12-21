ARCH           ?= $(shell uname -m)
GOARCH         ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(ARCH)))
VERSION        ?= 0.4.0
MAJOR_VERSION  ?= 21
MINOR_VERSION  ?= 7
ORACLE_VERSION ?= $(MAJOR_VERSION).$(MINOR_VERSION)
PKG_VERSION    ?= $(ORACLE_VERSION).0.0.0-1.el8.$(ARCH)
GLIBC_VERSION	 ?= 2.29-r0
LDFLAGS        := -X main.Version=$(VERSION)
GOFLAGS        := -ldflags "$(LDFLAGS) -s -w"
RPM_VERSION    ?= $(ORACLE_VERSION).0.0.0-1
ORA_RPM         = oracle-instantclient-basic-$(PKG_VERSION).rpm oracle-instantclient-devel-$(PKG_VERSION).rpm
LD_LIBRARY_PATH = /usr/lib/oracle/$(ORACLE_VERSION)/client64/lib
BUILD_ARGS      = --build-arg VERSION=$(VERSION) --build-arg ORACLE_VERSION=$(ORACLE_VERSION) \
                  --build-arg MAJOR_VERSION=$(MAJOR_VERSION)
LEGACY_TABLESPACE = --build-arg LEGACY_TABLESPACE=.legacy-tablespace
DIST_DIR        = oracledb_exporter.$(VERSION)-ora$(ORACLE_VERSION).linux-${GOARCH}
ARCHIVE         = oracledb_exporter.$(VERSION)-ora$(ORACLE_VERSION).linux-${GOARCH}.tar.gz

IMAGE_NAME     ?= iamseth/oracledb_exporter

export LD_LIBRARY_PATH ORACLE_VERSION

%.rpm:
	wget -q https://download.oracle.com/otn_software/linux/instantclient/217000/$@

download-rpms: $(ORA_RPM)

prereq: download-rpms
	@echo deps
	sudo apt-get update
	sudo apt-get install --no-install-recommends -qq libaio1 rpm
	sudo rpm -Uvh --nodeps --force oracle*rpm
	echo $(LD_LIBRARY_PATH) | sudo tee /etc/ld.so.conf.d/oracle.conf
	sudo ldconfig

oci.pc:
	sed "s/@ORACLE_VERSION@/$(ORACLE_VERSION)/g" oci8.pc.template | \
		sed "s/@MAJOR_VERSION@/$(MAJOR_VERSION)/g" > oci8.pc

linux: oci.pc
	@echo build linux
	mkdir -p ./dist/$(DIST_DIR)
	PKG_CONFIG_PATH=${PWD} GOOS=linux go build $(GOFLAGS) -o ./dist/$(DIST_DIR)/oracledb_exporter
	cp default-metrics.toml ./dist/$(DIST_DIR)
	(cd dist ; tar cfz $(ARCHIVE) $(DIST_DIR))

darwin: oci.pc
	@echo build darwin
	mkdir -p ./dist/oracledb_exporter.$(VERSION).darwin-${GOARCH}
	PKG_CONFIG_PATH=${PWD} GOOS=darwin go build $(GOFLAGS) -o ./dist/oracledb_exporter.$(VERSION).darwin-${GOARCH}/oracledb_exporter
	cp default-metrics.toml ./dist/oracledb_exporter.$(VERSION).darwin-${GOARCH}
	(cd dist ; tar cfz oracledb_exporter.$(VERSION).darwin-${GOARCH}.tar.gz oracledb_exporter.$(VERSION).darwin-${GOARCH})

local-build: linux

build: docker

deps:
	@PKG_CONFIG_PATH=${PWD} go get

test:
	@echo test
	@PKG_CONFIG_PATH=${PWD} go test $$(go list ./... | grep -v /vendor/)

clean:
	rm -rf ./dist sgerrand.rsa.pub glibc.apk oci8.pc

docker: ubuntu-image alpine-image oraclelinux-image

push-images:
	docker push $(IMAGE_NAME):$(VERSION)-oraclelinux
	docker push $(IMAGE_NAME):oraclelinux
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):$(VERSION)-alpine
	docker push $(IMAGE_NAME):alpine

sgerrand.rsa.pub:
	wget -q -O sgerrand.rsa.pub  https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub

glibc.apk:
	wget -q -O glibc-$(GLIBC_VERSION).apk https://github.com/sgerrand/alpine-pkg-glibc/releases/download/$(GLIBC_VERSION)/glibc-$(GLIBC_VERSION).apk

oraclelinux-image:
	docker build -f oraclelinux/Dockerfile $(BUILD_ARGS) -t "$(IMAGE_NAME):$(VERSION)-oraclelinux" .
	docker build -f oraclelinux/Dockerfile $(BUILD_ARGS) $(LEGACY_TABLESPACE) -t "$(IMAGE_NAME):$(VERSION)-oraclelinux_legacy-tablespace" .
	docker tag "$(IMAGE_NAME):$(VERSION)-oraclelinux" "$(IMAGE_NAME):oraclelinux"

ubuntu-image: $(ORA_RPM)
	docker build $(BUILD_ARGS) -t "$(IMAGE_NAME):$(VERSION)" .
	docker build $(BUILD_ARGS) $(LEGACY_TABLESPACE) -t "$(IMAGE_NAME):$(VERSION)_legacy-tablespace" .
	docker tag "$(IMAGE_NAME):$(VERSION)" "$(IMAGE_NAME):latest"

alpine-image: $(ORA_RPM) sgerrand.rsa.pub glibc.apk
	docker build -f alpine/Dockerfile $(BUILD_ARGS) -t "$(IMAGE_NAME):$(VERSION)-alpine" .
	docker build -f alpine/Dockerfile $(BUILD_ARGS) $(LEGACY_TABLESPACE) -t "$(IMAGE_NAME):$(VERSION)-alpine_legacy-tablespace" .
	docker tag "$(IMAGE_NAME):$(VERSION)-alpine" "$(IMAGE_NAME):alpine"

travis: oci.pc prereq deps test linux docker
	@true

.PHONY: build deps test clean docker travis oci.pc
