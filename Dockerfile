FROM golang:1.10-alpine3.7

RUN apk add -U ca-certificates curl git gcc musl-dev make
RUN curl -fsSL -o /usr/local/bin/dep https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 \
		&& chmod +x /usr/local/bin/dep

RUN mkdir -p $GOPATH/src/github.com/bgpat/ipdns
WORKDIR $GOPATH/src/github.com/bgpat/ipdns

COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only -v

ADD . ./
RUN make
RUN mv bin/ipdns /ipdns


#FROM alpine:3.7
FROM scratch
COPY --from=0 /ipdns /ipdns
EXPOSE 53 53/udp
ENTRYPOINT ["/ipdns"]
