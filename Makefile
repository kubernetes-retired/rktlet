# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GP := .gopath
PARENT := github.com/kubernetes-incubator
PKG := rktlet
PKGPATH := ${PWD}/${GP}/src/${PARENT}/${PKG}
GO_TEST_ARGS ?= 
export GOPATH=${PWD}/${GP}

ORG_PATH := github.com/kubernetes-incubator
REPO_PATH := ${ORG_PATH}/rktlet
VERSION := $(shell git describe --dirty --always)
GLDFLAGS := -X ${REPO_PATH}/version.Version=${VERSION}

all: build

build: path-setup
	cd "${PKGPATH}" && \
	go build -o bin/rktlet -ldflags "${GLDFLAGS}" ./cmd/server/main.go

path-setup:
	@if [ ! -d "${GP}" ]; then mkdir -p "${GP}/src/${PARENT}" "${GP}/pkg" "${GP}/bin"; fi && \
	if [ ! -e "${PKGPATH}" ]; then ln -s "${PWD}" "${PKGPATH}"; fi && \
	echo "Local GOPATH set up at ${GOPATH}"

build-in-rkt:
	sudo rkt run --uuid-file-save=/tmp/rktlet-build-uuid \
		docker://golang:1.7 --insecure-options=image,ondisk \
		--volume src,kind=host,source="$(shell pwd)" \
		--mount volume=src,target=/go/src/github.com/kubernetes-incubator/rktlet \
		--working-dir /go/src/github.com/kubernetes-incubator/rktlet \
		--exec=go -- build -o bin/container/rktlet ./cmd/server/main.go

glide:
	glide update --strip-vendor

generate: ./hack/bin/mockery path-setup
	go generate -x ./rktlet/...

test: path-setup
	cd "${PKGPATH}" && \
	go test ./rktlet/...

integ: path-setup
	@export RKTLET_TESTDIR=`mktemp -d` && \
	echo "Running integration tests, tempdir at $${RKTLET_TESTDIR}" && \
	cd "${PKGPATH}" && \
	sudo -E go test -v ./tests/... $(GO_TEST_ARGS) && \
	sudo rm -rf $${RKTLET_TESTDIR}


clean:
	rm -rf ./bin/rktlet ./hack/bin/mockery ./bin/container/rktlet ./${GP}


MOCKERY_SOURCES := $(shell find ./vendor/github.com/vektra/mockery/ -name '*.go')
./hack/bin/mockery: $(MOCKERY_SOURCES) path-setup
	cd "${PKGPATH}" && \
	go build -o ./hack/bin/mockery ./vendor/github.com/vektra/mockery/cmd/mockery

.PHONY: all build-in-rkt glide generate test integ clean path-setup
