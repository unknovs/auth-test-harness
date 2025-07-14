FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod ./

RUN apk --no-cache add git ca-certificates tzdata && \ 
    go mod download && \
    go generate ./...

COPY . ./

RUN go build -ldflags="-w -s" -tags 'netgo osusergo' -o publish/server . 
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
# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
ENTRYPOINT ["/server", "main"]
