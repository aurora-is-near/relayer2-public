GOCMD=go
GOFLAGS=-ldflags="-s -w"
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test

BIN_NAME=relayer
OUT_DIR=out
BIN_DIR=$(OUT_DIR)
CONF_DIR=$(OUT_DIR)/config
BUILD_INFO=$(OUT_DIR)/build.info

#@ run make with V=1 flag for verbose output
E ?= @
V ?= 0
ifeq ($(V), 0)
    Q = @
else
    Q =
endif

all: clean build

.PHONY: all

build-info:
	$(E)echo "$(BIN_NAME) build-info ..."
	$(Q)$(eval __build_branch:="branch: "$(shell git rev-parse --abbrev-ref HEAD))
	$(Q)$(eval __build_commit:="commit: "$(shell git rev-parse HEAD))
	$(Q)$(eval __build_tag:="tag: "$(shell git describe --exact-match $(git rev-parse HEAD) 2>/dev/null))
	$(E)echo " $(__build_branch)"
	$(E)echo " $(__build_commit)"
	$(E)echo " $(__build_tag)"

build: build-info
	$(E)echo "$(BIN_NAME) build ..."
	$(Q)mkdir -p $(OUT_DIR)
	$(Q)CGO_ENABLED=0 $(GOBUILD) $(GOFLAGS) -o $(BIN_DIR)/$(BIN_NAME)
	$(E)echo $(__build_branch) > $(BUILD_INFO)
	$(E)echo $(__build_commit) >> $(BUILD_INFO)
	$(E)echo $(__build_tag) >> $(BUILD_INFO)
	$(Q)cp -r config/ $(CONF_DIR)

clean:
	$(E)echo "$(BIN_NAME) clean ..."
	$(Q)rm -rf $(OUT_DIR)

test:
	$(E)echo "$(BIN_NAME) test ..."
	$(Q)$(GOTEST) -v ./...