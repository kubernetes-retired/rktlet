FROM debian:jessie
MAINTAINER Rktlet Developers <https://github.com/kubernetes-incubator/rktlet>

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    apt-get install -y libsystemd-journal-dev && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /journal/ /cri/
COPY ./bin/docker/journal2cri /usr/bin/journal2cri

VOLUME /journal
VOLUME /cri

CMD ["/usr/bin/journal2cri", "/journal", "/cri"]
