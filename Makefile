.PHONY: build server client

PROJECT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
OUTPUT_DIR := ${PROJECT_DIR}build/
GOPATH := $(shell go env GOPATH)

VERSION ?= v0.0.1

build: server client

client:
	@echo "Building client..."
	cd ${PROJECT_DIR} && \
	go build -v -trimpath \
		-ldflags "-X github.com/localvar/xuandb/pkg/version.version=${VERSION}" \
		-o build/bin/xuan ./cmd/xuan

server: generate
	@echo "Building server..."
	cd ${PROJECT_DIR} && \
	go build -v -trimpath -tags="xuandb_server"\
		-ldflags "-X github.com/localvar/xuandb/pkg/version.version=${VERSION}" \
		-o build/bin/xuand ./cmd/xuand

generate:
ifeq ($(wildcard ${GOPATH}/bin/goyacc),)
	@echo "Installing goyacc..."
	go install golang.org/x/tools/cmd/goyacc@v0.25.0
endif
	cd ${PROJECT_DIR} && \
	${GOPATH}/bin/goyacc -l -v pkg/parser/yacc.output -o pkg/parser/yacc.go pkg/parser/sql.y && \
	rm -f pkg/parser/yacc.output

clean:
	rm -f ${OUTPUT_DIR}bin/xuand
	rm -f ${OUTPUT_DIR}bin/xuan
	rm -f ${PROJECT_DIR}pkg/parser/yacc.output
