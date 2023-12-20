# Build the api binary
FROM golang:1.21.4 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/main.go cmd/main.go
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o api cmd/main.go

# Deploy
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/api .

ENTRYPOINT ["/api", "--ca-cert", "/certs/ca.crt", "--cert-key", "/certs/server.key", "--server-cert", "/certs/server.crt"]
