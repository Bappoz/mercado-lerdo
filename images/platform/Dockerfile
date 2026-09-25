# Platform binaries that run inside the sandbox (nexora-seed, observer-svc).
# Build context: repository root. Select the binary with --build-arg CMD=<name>.
FROM golang:1.27.1-alpine3.24@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build
ARG CMD
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/app ./cmd/${CMD}

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN adduser -D -H -u 10001 sesim
COPY --from=build /out/app /usr/local/bin/app
USER 10001
ENTRYPOINT ["/usr/local/bin/app"]
