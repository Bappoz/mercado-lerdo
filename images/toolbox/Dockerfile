# Player's investigation shell inside the sandbox network.
FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN apk add --no-cache bash curl jq bind-tools iproute2 procps-ng htop postgresql18-client less vim \
 && adduser -D -s /bin/bash -u 10001 player
USER 10001
WORKDIR /home/player
CMD ["sleep", "infinity"]
