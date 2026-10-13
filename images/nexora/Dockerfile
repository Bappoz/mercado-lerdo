# Nexora app (the player's code). Build context: nexora/.
# Dependencies come from vendor/, so the build needs no network.
FROM golang:1.27.2-alpine3.24@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
WORKDIR /src
COPY . .
# The build cache survives between builds, so a one-line change recompiles one package.
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -mod=vendor -trimpath -o /out/nexora ./cmd/nexora

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN adduser -D -H -u 10001 nexora
COPY --from=build /out/nexora /usr/local/bin/nexora
USER 10001
EXPOSE 8080 9091
HEALTHCHECK --interval=5s --timeout=2s --retries=6 CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/nexora"]
