# syntax=docker/dockerfile:1.7
FROM golang:1.25.7-alpine AS builder
WORKDIR /app
ARG TARGETARCH
ENV GOTOOLCHAIN=local CGO_ENABLED=1 CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN apk add --no-cache build-base wget unzip
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -ldflags "-w -s" -o build/pardis-ui main.go
RUN chmod +x DockerInitFiles.sh && ./DockerInitFiles.sh "$TARGETARCH"

FROM alpine
LABEL org.opencontainers.image.authors="pardisontop@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/build/ /app/
VOLUME [ "/etc/pardis-ui" ]
CMD [ "./pardis-ui" ]
