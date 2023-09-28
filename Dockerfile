FROM golang:1.20-alpine AS  builder
RUN apk update --no-cache && apk add \
        gcc \
        g++ \
        zlib \
        zlib-dev
WORKDIR  /go/src/github.com/zwindler/eventrouter
USER 0
COPY . .
RUN go build .

FROM golang:1.20-alpine
RUN addgroup -S eventroutergroup --gid 1010 && adduser -S eventrouter -G eventroutergroup --uid 1000
USER eventrouter
COPY --from=builder /go/src/github.com/zwindler/eventrouter/eventrouter /bin/eventrouter
CMD ["/bin/eventrouter", "-v", "3", "-logtostderr"]
LABEL version=v0.5.0
