# Builder
FROM golang:1.13.5 AS builder
ADD . /app
WORKDIR /app

ARG APP_NAME=s3s
ARG APP_RELEASE=dev
ARG BUILD_NUMBER
ARG COMMIT_HASH

RUN CGO_ENABLED=0 GOOS=linux make build RELEASE=${APP_RELEASE} BUILD_NUMBER=${BUILD_NUMBER} COMMIT_HASH=${COMMIT_HASH} OUTPUT=s3s

# Runner
FROM alpine:3.12.0

ENV USER=s3s
ENV UID=10001

RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates \
    && adduser \
    --disabled-password \
    --gecos "" \
    --no-create-home \
    --home "/nonexistent" \ 
    --shell "/sbin/nologin" \
    --uid "$UID" \
    "$USER"

COPY --from=builder /app/s3s /usr/local/bin/s3s

USER ${USER}

EXPOSE 8085

ENTRYPOINT [ "s3s" ]
