FROM golang:1.27-alpine AS build

WORKDIR /app

COPY go.mod ./

RUN apk --no-cache add git ca-certificates tzdata && \ 
    go mod download && \
    go generate ./...

COPY . ./

# Go's module and build caches are cache mounts, so a rebuild recompiles only what
# changed; neither is ever part of the image.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-w -s" -tags 'netgo osusergo' -o publish/server .
# && \
RUN    mkdir -p publish/etc/ssl/certs/ && \
    mkdir -p publish/usr/share/zoneinfo/ && \
    mkdir -p publish/certs/ && \
    cp /etc/ssl/certs/ca-certificates.crt publish/etc/ssl/certs/ && \
    cp -R /usr/share/zoneinfo publish/usr/share/

FROM scratch
WORKDIR /
COPY --from=build app/publish/ ./
EXPOSE 8080/tcp
ENV TZ=Europe/Riga

ENTRYPOINT ["/server", "main"]
