FROM golang:1.18.3-alpine3.16 AS  builder
RUN apk update --no-cache && apk add \
        gcc \
        g++ \
        zlib \
        zlib-dev
WORKDIR  /go/src/github.com/zwindler/eventrouter
USER 0
COPY . .
RUN go build .

FROM golang:1.18.3-alpine3.16
COPY --from=builder /go/src/github.com/zwindler/eventrouter/eventrouter /bin/eventrouter
CMD ["/bin/eventrouter", "-v", "3", "-logtostderr"]
LABEL version=v0.4.0
