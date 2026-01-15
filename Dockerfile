FROM caddy:2.10-builder AS builder

COPY caddy-anidb-handlers /caddy-anidb-handlers

RUN  --mount=type=cache,target=/go/pkg/mod \
     --mount=type=cache,target=/root/.cache/go-build \
     xcaddy build \
     --with github.com/trajano/anidb-proxy/caddy-anidb-handlers=/caddy-anidb-handlers \
     --with github.com/caddyserver/cache-handler@v0.16.0 \
     --with github.com/darkweak/storages/nuts/caddy@v0.0.16 \
     --with github.com/darkweak/storages/simplefs/caddy@v0.0.16

FROM busybox:1.36.1-uclibc AS staging
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile

FROM staging AS test
RUN caddy validate --config  /etc/caddy/Caddyfile

FROM staging
HEALTHCHECK --start-period=10s --start-interval=1s CMD wget -q --spider http://localhost/ || exit 1
CMD [ "/usr/bin/caddy", "run", "--config", "/etc/caddy/Caddyfile" ]
