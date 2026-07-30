# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/microbit-webhook-bridge .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

COPY --from=build /out/microbit-webhook-bridge /usr/local/bin/microbit-webhook-bridge

ENTRYPOINT ["/usr/local/bin/microbit-webhook-bridge"]
