FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/eventbooker ./cmd/server

FROM golang:1.25-alpine AS debug-builder

WORKDIR /src

RUN apk add --no-cache git && go install github.com/go-delve/delve/cmd/dlv@v1.26.1

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags="all=-N -l" -o /out/eventbooker ./cmd/server

FROM alpine:3.21 AS release

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/eventbooker /app/eventbooker
COPY configs /app/configs
COPY web /app/web

EXPOSE 8080

ENTRYPOINT ["/app/eventbooker"]

FROM alpine:3.21 AS debug

WORKDIR /app

RUN apk add --no-cache ca-certificates libc6-compat

COPY --from=debug-builder /out/eventbooker /app/eventbooker
COPY --from=debug-builder /go/bin/dlv /usr/local/bin/dlv
COPY configs /app/configs
COPY web /app/web

EXPOSE 8080 40000

ENTRYPOINT ["dlv", "exec", "/app/eventbooker", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient"]
