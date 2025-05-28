FROM caddy:builder AS builder
RUN xcaddy build \
     --with github.com/caddyserver/cache-handler \
     --with github.com/darkweak/storages/nuts/caddy \
     --with github.com/darkweak/storages/redis/caddy

FROM busybox:1.36.1-uclibc AS staging
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile

FROM staging AS test
RUN caddy validate --config  /etc/caddy/Caddyfile

FROM staging
HEALTHCHECK --start-period=10s --start-interval=1s CMD wget -q --spider http://localhost/ || exit 1
CMD [ "/usr/bin/caddy", "run", "--config", "/etc/caddy/Caddyfile" ]