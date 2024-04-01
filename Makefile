ARCH           ?= $(shell uname -m)
OS_TYPE        ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH_TYPE      ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(ARCH)))
GOOS           ?= $(shell go env GOOS)
GOARCH         ?= $(shell go env GOARCH)
VERSION        ?= 0.6.0
LDFLAGS        := -X main.Version=$(VERSION)
GOFLAGS        := -ldflags "$(LDFLAGS) -s -w"
BUILD_ARGS      = --build-arg VERSION=$(VERSION)
LEGACY_TABLESPACE = --build-arg LEGACY_TABLESPACE=.legacy-tablespace
OUTDIR          = ./dist
LINTER_VERSION ?= v1.55.2
LINTER_IMAGE   ?= docker.io/golangci/golangci-lint:$(LINTER_VERSION)

ifeq ($(shell command -v podman 2> /dev/null),)
    DOCKER_CMD  = docker
else
    DOCKER_CMD  = podman
endif

IMAGE_NAME     ?= iamseth/oracledb_exporter
IMAGE_ID       ?= $(IMAGE_NAME):$(VERSION)
IMAGE_ID_LATEST?= $(IMAGE_NAME):latest
RELEASE        ?= true

UBUNTU_BASE_IMAGE       ?= docker.io/library/ubuntu:24.04
ORACLE_LINUX_BASE_IMAGE ?= docker.io/library/oraclelinux:9-slim
ALPINE_BASE_IMAGE       ?= docker.io/library/alpine:3.19

ifeq ($(GOOS), windows)
EXT?=.exe
else
EXT?=
endif

export LD_LIBRARY_PATH

version:
	@echo "$(VERSION)"

.PHONY: go-build
go-build:
	@echo "Build $(OS_TYPE)"
	mkdir -p $(OUTDIR)/oracledb_exporter-$(VERSION).$(GOOS)-$(GOARCH)/
	go build $(GOFLAGS) -o $(OUTDIR)/oracledb_exporter-$(VERSION).$(GOOS)-$(GOARCH)/oracledb_exporter$(EXT)
	cp default-metrics.toml $(OUTDIR)/$(DIST_DIR)
	(cd dist ; tar cfz oracledb_exporter-$(VERSION).$(GOOS)-$(GOARCH).tar.gz oracledb_exporter-$(VERSION).$(GOOS)-$(GOARCH))

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
	mkdir -p ~/.cache/golangci-lint/$(LINTER_VERSION)
	$(DOCKER_CMD) run --rm -v $$PWD:/app -v ~/.cache/golangci-lint/$(LINTER_VERSION):/root/.cache -w /app \
	                $(LINTER_IMAGE) golangci-lint run -v

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

oraclelinux-image:
	if DOCKER_CLI_EXPERIMENTAL=enabled $(DOCKER_CMD) manifest inspect "$(IMAGE_ID)-oraclelinux" > /dev/null; then \
		echo "Image \"$(IMAGE_ID)-oraclelinux\" already exists on ghcr.io"; \
	else \
		$(DOCKER_CMD) build --progress=plain $(BUILD_ARGS) -t "$(IMAGE_ID)-oraclelinux" --build-arg BASE_IMAGE=$(ORACLE_LINUX_BASE_IMAGE) . && \
		$(DOCKER_CMD) build --progress=plain $(BUILD_ARGS) $(LEGACY_TABLESPACE) -t "$(IMAGE_ID)-oraclelinux_legacy-tablespace" --build-arg BASE_IMAGE=$(ORACLE_LINUX_BASE_IMAGE) . && \
		$(DOCKER_CMD) tag "$(IMAGE_ID)-oraclelinux" "$(IMAGE_NAME):oraclelinux"; \
	fi

push-oraclelinux-image:
	$(DOCKER_CMD) push $(IMAGE_ID)-oraclelinux
ifeq ("$(RELEASE)", "true")
	$(DOCKER_CMD) push "$(IMAGE_NAME):oraclelinux"
	$(DOCKER_CMD) push "$(IMAGE_ID)-oraclelinux_legacy-tablespace"
endif

sign-oraclelinux-image:
ifneq ("$(wildcard cosign.key)","")
	cosign sign --key cosign.key $(IMAGE_ID)-oraclelinux
else
	@echo "Can't find cosign.key file"
endif

ubuntu-image:
	if DOCKER_CLI_EXPERIMENTAL=enabled $(DOCKER_CMD) manifest inspect "$(IMAGE_ID)" > /dev/null; then \
		echo "Image \"$(IMAGE_ID)\" already exists on ghcr.io"; \
	else \
		$(DOCKER_CMD) build --progress=plain $(BUILD_ARGS) --build-arg BASE_IMAGE=$(UBUNTU_BASE_IMAGE) -t "$(IMAGE_ID)" . && \
		$(DOCKER_CMD) build --progress=plain $(BUILD_ARGS) --build-arg BASE_IMAGE=$(UBUNTU_BASE_IMAGE) $(LEGACY_TABLESPACE) -t "$(IMAGE_ID)_legacy-tablespace" . && \
		$(DOCKER_CMD) tag "$(IMAGE_ID)" "$(IMAGE_ID_LATEST)"; \
	fi

push-ubuntu-image:
	$(DOCKER_CMD) push $(IMAGE_ID)
ifeq ("$(RELEASE)", "true")
	$(DOCKER_CMD) push "$(IMAGE_ID_LATEST)"
	$(DOCKER_CMD) push "$(IMAGE_ID)_legacy-tablespace"
endif

sign-ubuntu-image:
ifneq ("$(wildcard cosign.key)","")
	cosign sign --key cosign.key $(IMAGE_ID)
	cosign sign --key cosign.key $(IMAGE_ID_LATEST)
else
	@echo "Can't find cosign.key file"
endif

alpine-image:
	if DOCKER_CLI_EXPERIMENTAL=enabled $(DOCKER_CMD) manifest inspect "$(IMAGE_ID)-alpine" > /dev/null; then \
		echo "Image \"$(IMAGE_ID)-alpine\" already exists on ghcr.io"; \
	else \
		$(DOCKER_CMD) build --progress=plain $(BUILD_ARGS) -t "$(IMAGE_ID)-alpine" --build-arg BASE_IMAGE=$(ALPINE_BASE_IMAGE) . && \
		$(DOCKER_CMD) build --progress=plain $(BUILD_ARGS) $(LEGACY_TABLESPACE) --build-arg BASE_IMAGE=$(ALPINE_BASE_IMAGE) -t "$(IMAGE_ID)-alpine_legacy-tablespace" . && \
		$(DOCKER_CMD) tag "$(IMAGE_ID)-alpine" "$(IMAGE_NAME):alpine"; \
	fi

push-alpine-image:
	$(DOCKER_CMD) push $(IMAGE_ID)-alpine
ifeq ("$(RELEASE)", "true")
	$(DOCKER_CMD) push "$(IMAGE_NAME):alpine"
endif

sign-alpine-image:
ifneq ("$(wildcard cosign.key)","")
	cosign sign --key cosign.key $(IMAGE_ID)-alpine
else
	@echo "Can't find cosign.key file"
endif

.PHONY: version build deps go-test clean docker
