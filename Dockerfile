FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY . /app
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build

FROM alpine:latest
COPY --from=builder /app/mosdns /usr/local/bin/mosdns
CMD ["/usr/local/bin/mosdns"]
