# syntax=docker/dockerfile:1

FROM golang:1.25-trixie AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG COMMIT_ID=docker
WORKDIR /workspace
ADD src /workspace/src
WORKDIR /workspace/src
RUN go mod tidy
RUN go get
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.GIT_COMMIT=${COMMIT_ID}" -o /workspace/dn42-wiki-go ./

FROM debian:trixie-slim AS runtime
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates git && \
    rm -rf /var/lib/apt/lists/*
RUN useradd --system --home /app --shell /usr/sbin/nologin wiki
WORKDIR /app
COPY --from=builder /workspace/dn42-wiki-go ./dn42-wiki-go
COPY template ./template
COPY config.example.json ./config.json
RUN mkdir -p /app/dist /app/repo && \
    chown -R wiki:wiki /app
VOLUME ["/app/dist", "/app/repo"]
EXPOSE 8080
USER wiki
ENTRYPOINT ["./dn42-wiki-go"]
CMD ["--config", "/app/config.json"]
