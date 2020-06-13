FROM golang:alpine AS builder

RUN apk add --no-cache git

COPY . /go/src/github.com/juanhuttemann/nitr-agent/
WORKDIR /go/src/github.com/juanhuttemann/nitr-agent/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o nitr .

EXPOSE 8000
CMD ["./nitr", "&"]