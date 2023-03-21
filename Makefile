ARCH           ?= $(shell uname -m)
OS_TYPE        ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH_TYPE      ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(ARCH)))
GOOS           ?= $(shell go env GOOS)
GOARCH         ?= $(shell go env GOARCH)
VERSION        ?= 0.4.4
MAJOR_VERSION  ?= 21
MINOR_VERSION  ?= 8
ORACLE_VERSION ?= $(MAJOR_VERSION).$(MINOR_VERSION)
ORACLE_IMAGE   ?= ghcr.io/oracle/oraclelinux8-instantclient:$(MAJOR_VERSION)
PKG_VERSION    ?= $(ORACLE_VERSION).0.0.0-1.el8.$(ARCH)
GLIBC_VERSION	 ?= 2.35-r0
LDFLAGS        := -X main.Version=$(VERSION)
GOFLAGS        := -ldflags "$(LDFLAGS) -s -w"
RPM_VERSION    ?= $(ORACLE_VERSION).0.0.0-1
ORA_RPM         = oracle-instantclient-basic-$(PKG_VERSION).rpm oracle-instantclient-devel-$(PKG_VERSION).rpm
LD_LIBRARY_PATH = /usr/lib/oracle/$(ORACLE_VERSION)/client64/lib
BUILD_ARGS      = --build-arg VERSION=$(VERSION) --build-arg ORACLE_VERSION=$(ORACLE_VERSION) \
                  --build-arg MAJOR_VERSION=$(MAJOR_VERSION) --build-arg ORACLE_IMAGE=$(ORACLE_IMAGE)
LEGACY_TABLESPACE = --build-arg LEGACY_TABLESPACE=.legacy-tablespace
DIST_DIR        = oracledb_exporter-$(VERSION)-ora$(ORACLE_VERSION).$(OS_TYPE)-$(ARCH_TYPE)
ARCHIVE         = oracledb_exporter-$(VERSION)-ora$(ORACLE_VERSION).$(OS_TYPE)-$(ARCH_TYPE).tar.gz

IMAGE_NAME     ?= iamseth/oracledb_exporter
IMAGE_ID       ?= $(IMAGE_NAME):$(VERSION)
IMAGE_ID_LATEST?= $(IMAGE_NAME):latest
RELEASE        ?= true

export LD_LIBRARY_PATH ORACLE_VERSION

version:
	@echo "$(VERSION)"

oracle-version:
	@echo "$(ORACLE_VERSION)"

%.rpm:
	wget -q "https://download.oracle.com/otn_software/linux/instantclient/$(MAJOR_VERSION)$(MINOR_VERSION)000/$@"

download-rpms: $(ORA_RPM)
	@true

prereq: download-rpms
	@echo deps
	sudo apt-get update
	sudo apt-get install --no-install-recommends -qq libaio1 rpm alien
	sudo alien -i oracle*.rpm || sudo rpm -Uvh --nodeps --force oracle*.rpm
	echo $(LD_LIBRARY_PATH) | sudo tee /etc/ld.so.conf.d/oracle.conf
	sudo ldconfig

oci.pc:
	sed "s/@ORACLE_VERSION@/$(ORACLE_VERSION)/g" oci8.pc.template | \
	sed "s/@MAJOR_VERSION@/$(MAJOR_VERSION)/g" > oci8.pc

go-build: oci.pc
	@echo "Build $(OS_TYPE)"
	mkdir -p ./dist/$(DIST_DIR)
	PKG_CONFIG_PATH=${PWD} GOOS=$(OS_TYPE) GOARCH=$(ARCH_TYPE) go build $(GOFLAGS) -o ./dist/$(DIST_DIR)/oracledb_exporter
	cp default-metrics.toml ./dist/$(DIST_DIR)
	(cd dist ; tar cfz $(ARCHIVE) $(DIST_DIR))

go-lint:
	@echo "Linting codebase"
	docker run --rm -v $(shell pwd):/app -v ~/.cache/golangci-lint/v1.50.1:/root/.cache -w /app golangci/golangci-lint:v1.50.1 golangci-lint run -v

local-build: go-build
	@true

build: docker
	@true

deps:
	@PKG_CONFIG_PATH=${PWD} go get

go-test:
	@echo "Run tests"
	@PKG_CONFIG_PATH=${PWD} GOOS=$(OS_TYPE) GOARCH=$(ARCH_TYPE) go test -coverprofile="test-coverage.out" $$(go list ./... | grep -v /vendor/)

clean:
	rm -rf ./dist sgerrand.rsa.pub glibc-*.apk oracle-*.rpm oci8.pc

docker: ubuntu-image alpine-image oraclelinux-image

push-images:
	@make --no-print-directory push-ubuntu-image
	@make --no-print-directory push-oraclelinux-image
	@make --no-print-directory push-alpine-image

glibc.apk:
	wget -q -O sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub
	wget -q -O glibc-$(GLIBC_VERSION).apk https://github.com/sgerrand/alpine-pkg-glibc/releases/download/$(GLIBC_VERSION)/glibc-$(GLIBC_VERSION).apk

oraclelinux-image:
	if DOCKER_CLI_EXPERIMENTAL=enabled docker manifest inspect "$(IMAGE_ID)-oraclelinux" > /dev/null; then \
		echo "Image \"$(IMAGE_ID)-oraclelinux\" already exists on ghcr.io"; \
	else \
		docker build --progress=plain -f oraclelinux/Dockerfile $(BUILD_ARGS) -t "$(IMAGE_ID)-oraclelinux" . && \
		docker build --progress=plain -f oraclelinux/Dockerfile $(BUILD_ARGS) $(LEGACY_TABLESPACE) -t "$(IMAGE_ID)-oraclelinux_legacy-tablespace" . && \
		docker tag "$(IMAGE_ID)-oraclelinux" "$(IMAGE_NAME):oraclelinux"; \
	fi

push-oraclelinux-image:
	docker push $(IMAGE_ID)-oraclelinux
ifeq ("$(RELEASE)", "true")
	docker push "$(IMAGE_NAME):oraclelinux"
	docker push "$(IMAGE_ID)-oraclelinux_legacy-tablespace"
endif

sign-oraclelinux-image:
ifneq ("$(wildcard cosign.key)","")
	cosign sign --key cosign.key $(IMAGE_ID)-oraclelinux
else
	@echo "Can't find cosign.key file"
endif

ubuntu-image: $(ORA_RPM)
	if DOCKER_CLI_EXPERIMENTAL=enabled docker manifest inspect "$(IMAGE_ID)" > /dev/null; then \
		echo "Image \"$(IMAGE_ID)\" already exists on ghcr.io"; \
	else \
		docker build --progress=plain $(BUILD_ARGS) -t "$(IMAGE_ID)" . && \
		docker build --progress=plain $(BUILD_ARGS) $(LEGACY_TABLESPACE) -t "$(IMAGE_ID)_legacy-tablespace" . && \
		docker tag "$(IMAGE_ID)" "$(IMAGE_ID_LATEST)"; \
	fi

push-ubuntu-image:
	docker push $(IMAGE_ID)
ifeq ("$(RELEASE)", "true")
	docker push "$(IMAGE_ID_LATEST)"
	docker push "$(IMAGE_ID)_legacy-tablespace"
endif

sign-ubuntu-image:
ifneq ("$(wildcard cosign.key)","")
	cosign sign --key cosign.key $(IMAGE_ID)
	cosign sign --key cosign.key $(IMAGE_ID_LATEST)
else
	@echo "Can't find cosign.key file"
endif

alpine-image: $(ORA_RPM)
	if DOCKER_CLI_EXPERIMENTAL=enabled docker manifest inspect "$(IMAGE_ID)-alpine" > /dev/null; then \
		echo "Image \"$(IMAGE_ID)-alpine\" already exists on ghcr.io"; \
	else \
		docker build --progress=plain -f alpine/Dockerfile $(BUILD_ARGS) -t "$(IMAGE_ID)-alpine" . && \
		docker build --progress=plain -f alpine/Dockerfile $(BUILD_ARGS) $(LEGACY_TABLESPACE) -t "$(IMAGE_ID)-alpine_legacy-tablespace" . && \
		docker tag "$(IMAGE_ID)-alpine" "$(IMAGE_NAME):alpine"; \
	fi

push-alpine-image:
	docker push $(IMAGE_ID)-alpine
ifeq ("$(RELEASE)", "true")
	docker push "$(IMAGE_NAME):alpine"
endif

sign-alpine-image:
ifneq ("$(wildcard cosign.key)","")
	cosign sign --key cosign.key $(IMAGE_ID)-alpine
else
	@echo "Can't find cosign.key file"
endif

travis: oci.pc prereq deps go-test go-build docker
	@true

.PHONY: version build deps go-test clean docker travis glibc.apk oci.pc
