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

.PHONY: all glide generate test clean

all:
	go build -o bin/rktlet ./cmd/server/main.go

glide:
	glide update --strip-vendor

generate: ./hack/bin/mockery
	go generate -x ./rktlet/...

test:
	go test ./rktlet/...

clean:
	rm -f ./bin/rktlet ./hack/bin/mockery

MOCKERY_SOURCES := $(shell find ./vendor/github.com/vektra/mockery/ -name '*.go')
./hack/bin/mockery: $(MOCKERY_SOURCES)
	go build -o ./hack/bin/mockery ./vendor/github.com/vektra/mockery/cmd/mockery
