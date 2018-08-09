FROM golang:1.10-alpine3.8 AS builder

WORKDIR /go/src/github.com/saj/redis-junk

COPY vendor vendor/
COPY *.go ./

RUN go build .


FROM alpine:3.8

RUN addgroup redis-junk
RUN adduser -D -G redis-junk redis-junk

COPY --from=builder \
  /go/src/github.com/saj/redis-junk/redis-junk \
  /usr/local/bin/redis-junk

USER redis-junk:redis-junk

ENTRYPOINT ["/usr/local/bin/redis-junk"]
