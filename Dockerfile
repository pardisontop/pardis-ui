FROM golang:1.25.7-alpine AS builder
WORKDIR /app
ARG TARGETARCH
ENV GOTOOLCHAIN=local
RUN apk --no-cache --update add build-base gcc wget unzip
COPY . .
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN go build -ldflags "-w -s" -o build/pardis-ui main.go
RUN chmod +x DockerInitFiles.sh && ./DockerInitFiles.sh "$TARGETARCH"

FROM alpine
LABEL org.opencontainers.image.authors="pardisontop@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app

RUN apk add ca-certificates tzdata

COPY --from=builder /app/build/ /app/
VOLUME [ "/etc/pardis-ui" ]
CMD [ "./pardis-ui" ]
