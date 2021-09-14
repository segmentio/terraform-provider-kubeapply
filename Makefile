ifndef VERSION_REF
	VERSION_REF ?= $(shell git describe --tags --always --dirty="-dev")
endif

LDFLAGS := -ldflags='-X "main.VersionRef=$(VERSION_REF)"'

GOFILES = $(shell find . -iname '*.go' | grep -v -e vendor -e _modules -e _cache -e /data/)

# If / when this provider begins normal development, this should be dynamically determined.
PROVIDER_VERSION = 0.0.1
OS = $(shell go env GOOS)
ARCH = $(shell go env GOARCH)
TF_PLUGINS_MIRROR_DIR = $(HOME)/.terraform.d/plugins

# Provider targets
.PHONY: terraform-provider-kubeapply
terraform-provider-kubeapply:
	go build -o build/terraform-provider-kubeapply $(LDFLAGS) .

# https://www.terraform.io/docs/cli/config/config-file.html#implied-local-mirror-directories
.PHONY: terraform-provider-mirror-install
terraform-provider-mirror-install: terraform-provider-kubeapply
	mkdir -p $(TF_PLUGINS_MIRROR_DIR)/segment.io/kubeapply/kubeapply/$(PROVIDER_VERSION)/$(OS)_$(ARCH)
	cp build/terraform-provider-kubeapply $(TF_PLUGINS_MIRROR_DIR)/segment.io/kubeapply/kubeapply/$(PROVIDER_VERSION)/$(OS)_$(ARCH)

# Helper targets
.PHONY: kadiff
kadiff:
	go build -o build/kadiff $(LDFLAGS) ./cmd/kadiff

.PHONY: install-kadiff
install-kadiff:
	go install $(LDFLAGS) ./cmd/kadiff

.PHONY: kaexpand
kaexpand:
	go build -o build/kaexpand $(LDFLAGS) ./cmd/kaexpand

.PHONY: install-kaexpand
install-kaexpand:
	go install $(LDFLAGS) ./cmd/kaexpand

# Test and formatting targets
.PHONY: test
test: vet
	go test -p 1 -count 1 -cover ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: docs
docs:
	tfplugindocs

.PHONY: clean
clean:
	rm -Rf build vendor
