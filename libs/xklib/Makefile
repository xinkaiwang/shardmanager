SHELL=/bin/bash

# Important targets
.PHONY: clean FORCE
all: hello

GO_BIN ?= go
# BIBLE_IMG=bible

# PROTO_INDIR=$(CURDIR)/proto
# PROTO_OUTDIR=$(CURDIR)/api
# include $(PROTO_INDIR)/Makefile

MAJOR_VERSION=0
MINOR_VERSION=0
PATCH_VERSION=1

hello:
	CGO_ENABLED=1 $(GO_BIN) build $(GO_GCFLAGS) -o build/hello ./cmd/hello

# test: protogen generate
# 	@$(GO_BIN) test -timeout 10s ./...