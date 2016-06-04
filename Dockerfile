FROM golang:1.7
MAINTAINER Eric Holmes <eric@remind101.com>

LABEL version 0.10.0

ADD . /go/src/github.com/remind101/empire
WORKDIR /go/src/github.com/remind101/empire
RUN GO15VENDOREXPERIMENT=1 go install ./cmd/empire

ENTRYPOINT ["/go/bin/empire"]
CMD ["server"]

EXPOSE 8080
