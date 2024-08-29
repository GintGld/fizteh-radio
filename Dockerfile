FROM golang:alpine AS builder

ENV CGO_ENABLED=1

WORKDIR /build

# CGo lib to use sqlite
RUN apk add --update gcc musl-dev sqlite-dev

ADD go.mod .

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

RUN --mount=type=cache,target=/go/pkg/mod/ \
    go build -o radio ./cmd/radio
RUN --mount=type=cache,target=/go/pkg/mod/ \
    go build -o migrator ./cmd/migrator

FROM alpine AS final

RUN --mount=type=cache,target=/var/cache/apk \
    apk --update add \
        ca-certificates \
        tzdata \
        ffmpeg \
        && \
        update-ca-certificates

WORKDIR /radio

# copy executables
COPY --from=builder /build/radio /radio/radio
COPY --from=builder /build/migrator /radio/migrator

# TODO: move this copies to external volumes

# copy utils
COPY config/prod.yaml .
COPY migrations migrations
COPY scripts scripts

EXPOSE 8082

ENTRYPOINT [ "sh", "./scripts/run.sh" ]