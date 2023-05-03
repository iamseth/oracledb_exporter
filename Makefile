ARCH           ?= $(shell uname -m)
OS_TYPE        ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH_TYPE      ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(ARCH)))
GOOS           ?= $(shell go env GOOS)
GOARCH         ?= $(shell go env GOARCH)
VERSION        ?= 0.5.0
MAJOR_VERSION  ?= 21
MINOR_VERSION  ?= 8
ORACLE_VERSION ?= $(MAJOR_VERSION).$(MINOR_VERSION)
ORACLE_IMAGE   ?= ghcr.io/oracle/oraclelinux8-instantclient:$(MAJOR_VERSION)
PKG_VERSION    ?= $(ORACLE_VERSION).0.0.0-1.el8.$(ARCH)
GLIBC_VERSION	 ?= 2.35-r0
LDFLAGS        := -X main.Version=$(VERSION)
GOFLAGS        := -ldflags "$(LDFLAGS) -s -w"
BUILD_ARGS      = --build-arg VERSION=$(VERSION) --build-arg ORACLE_VERSION=$(ORACLE_VERSION) \
                  --build-arg MAJOR_VERSION=$(MAJOR_VERSION) --build-arg ORACLE_IMAGE=$(ORACLE_IMAGE)
LEGACY_TABLESPACE = --build-arg LEGACY_TABLESPACE=.legacy-tablespace
OUTDIR          = ./dist

IMAGE_NAME     ?= iamseth/oracledb_exporter
IMAGE_ID       ?= $(IMAGE_NAME):$(VERSION)
IMAGE_ID_LATEST?= $(IMAGE_NAME):latest
RELEASE        ?= true

ifeq ($(GOOS), windows)
EXT?=.exe
else
EXT?=
endif

export LD_LIBRARY_PATH ORACLE_VERSION

version:
	@echo "$(VERSION)"

oracle-version:
	@echo "$(ORACLE_VERSION)"

.PHONY: go-build
go-build:
	@echo "Build $(OS_TYPE)"
	mkdir -p $(OUTDIR)/oracledb_exporter-$(VERSION)-ora$(ORACLE_VERSION).$(GOOS)-$(GOARCH)/
	go build $(GOFLAGS) -o $(OUTDIR)/oracledb_exporter-$(VERSION)-ora$(ORACLE_VERSION).$(GOOS)-$(GOARCH)/oracledb_exporter$(EXT)
	cp default-metrics.toml $(OUTDIR)/$(DIST_DIR)
	(cd dist ; tar cfz oracledb_exporter-$(VERSION)-ora$(ORACLE_VERSION).$(GOOS)-$(GOARCH).tar.gz oracledb_exporter-$(VERSION)-ora$(ORACLE_VERSION).$(GOOS)-$(GOARCH))

.PHONY: go-build-linux-amd64
go-build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) go-build -j2

.PHONY: go-build-linux-arm64
go-build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(MAKE) go-build -j2

.PHONY: go-build-darwin-amd64
go-build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(MAKE) go-build -j2

.PHONY: go-build-darwin-arm64
go-build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(MAKE) go-build -j2

.PHONY: go-build-windows-amd64
go-build-windows-amd64:
	GOOS=windows GOARCH=amd64 $(MAKE) go-build -j2

.PHONY: go-build-windows-x86
go-build-windows-x86:
	GOOS=windows GOARCH=386 $(MAKE) go-build -j2

go-lint:
	@echo "Linting codebase"
	docker run --rm -v $(shell pwd):/app -v ~/.cache/golangci-lint/v1.50.1:/root/.cache -w /app golangci/golangci-lint:v1.50.1 golangci-lint run -v

local-build: go-build
	@true

build: docker
	@true

deps:
	go get

go-test:
	@echo "Run tests"
	GOOS=$(OS_TYPE) GOARCH=$(ARCH_TYPE) go test -coverprofile="test-coverage.out" $$(go list ./... | grep -v /vendor/)

clean:
	rm -rf ./dist sgerrand.rsa.pub glibc-*.apk oracle-*.rpm

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

ubuntu-image:
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

alpine-image:
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

.PHONY: version build deps go-test clean docker glibc.apk
