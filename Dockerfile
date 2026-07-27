FROM golang:alpine AS builder

RUN apk add --no-cache git

COPY . /go/src/github.com/bitcav/nitr/
WORKDIR /go/src/github.com/bitcav/nitr/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o nitr .

EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://localhost:8000/health || exit 1
CMD ["./nitr"]