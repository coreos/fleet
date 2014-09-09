FROM deis/go:latest

WORKDIR /go/src/github.com/coreos/fleet
ENV PATH /go/src/github.com/coreos/fleet/bin:$PATH
ADD . /go/src/github.com/coreos/fleet
RUN ./build
