# Connection pooler from the catalog (internal/topology). Configured from the
# environment at start; the config lives on a tmpfs, never in the image.
FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN apk add --no-cache pgbouncer \
 && adduser -D -H -s /sbin/nologin -u 10002 pooler
COPY --chmod=0755 entrypoint.sh /usr/local/bin/entrypoint.sh
USER 10002
EXPOSE 6432
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
