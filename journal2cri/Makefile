.PHONY: all docker docker-push release clean

TAG=0.0.1
SOURCES := $(shell find ./journal2cri ./cmd -name '*.go')

all: ./bin/journal2cri

./bin/journal2cri: $(SOURCES)
	go build -o bin/journal2cri ./cmd/journal2cri/main.go

./bin/docker/journal2cri:
	# Build context of parent directory to pick up our vendor folder
	docker build -t localhost/journal2cri-builder:$(TAG) -f Dockerfile.build ..
	docker run -v \
		"$(shell pwd)/bin/docker/":/go/src/github.com/kubernetes-incubator/rktlet/journal2cri/bin \
		localhost/journal2cri-builder:$(TAG)

docker: ./bin/docker/journal2cri
	docker build -t quay.io/coreos/rktlet-journal2cri:$(TAG) .

docker-push: docker
	docker push quay.io/coreos/rktlet-journal2cri:$(TAG)

release: docker-push

clean:
	rm -f ./bin/journal2cri
	rm -f ./bin/docker/journal2cri
