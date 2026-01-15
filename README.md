# AniDB proxy
This is a caching proxy for AniDB to prevent the rate limits by ensuring requests are cached.  This is intended throttle the performance to prevent bans.

## Endpoints

* `/api/anime-titles.dat.gz` maps to https://anidb.net/api/anime-titles.dat.gz
* `/api/anime-titles.xml.gz` maps to https://anidb.net/api/anime-titles.xml.gz
* `/httpapi` maps to http://api.anidb.net:9001/httpapi documented in https://wiki.anidb.net/HTTP_API_Definition
* `/images/main/*` maps to https://cdn.anidb.net/images/main/

## httpapi handling

The `/httpapi` route applies two extra behaviors:

* Upstream calls are rate-limited so only one request is sent every 2 seconds (cache hits are returned immediately).
* If the first bytes of the response (after gzip decoding) contain `<error`, the proxy sets `Cache-Control: no-store` to avoid caching error responses.
* `request=anime` only keys against `aid` and drops the other query parameters for the key

You can configure these environment variables:

- `HTTPAPI_MIN_DURATION` (default: `5s`) — the minimum spacing between upstream calls enforced by the `min_duration` handler.
- `HTTPAPI_BACKEND_TIMEOUT` (default: `300s`) — the cache backend timeout used for upstream backend requests.
- `HTTPAPI_BACKEND` (default: `http://api.anidb.net:9001`) — the upstream backend URL; you can point this at another proxy instance to chain with a friend and balance request limits.

Important: `HTTPAPI_BACKEND_TIMEOUT` must be at least as large as `HTTPAPI_MIN_DURATION`, and in practice should be larger because multiple requests may queue behind the first and each queued request increases the time the backend needs to serve them. A recommended tuning starting point is to set `HTTPAPI_BACKEND_TIMEOUT` to 100× `HTTPAPI_MIN_DURATION`.

Optionally, set a jitter factor (a small fractional random delay applied to `min_duration`) via the environment variable `HTTPAPI_MIN_DURATION_JITTER` (default: `0.01`). Example (docker-compose):

```yaml
services:
  anidb-proxy:
    image: ghcr.io/trajano/anidb-proxy:latest
    environment:
      - HTTPAPI_MIN_DURATION=5s
      - HTTPAPI_BACKEND_TIMEOUT=300s
      - HTTPAPI_MIN_DURATION_JITTER=0.01
```

The `min_duration` handler also supports optional Caddyfile settings:
- `wait_threshold` (default: `5s`) — maximum delay before the handler responds or proceeds.
- `wait_mode` (default: `redirect`) — `redirect`, `retry-after`, or `wait`.

Note: when `wait_mode` is set to `wait`, two requests for the same ID can both reach the upstream and the second request will not leverage the cache.

Additional mappings provided against `/httpapi/` so it will be a single root.
* `/httpapi/anime-titles.dat.gz` maps to https://anidb.net/api/anime-titles.dat.gz
* `/httpapi/anime-titles.xml.gz` maps to https://anidb.net/api/anime-titles.xml.gz
* `/httpapi/images/main/*` maps to https://cdn.anidb.net/images/main/
* `/httpapi/search` maps to the [search server](https://anisearch.outrance.pl/doc.html) though it's TTL is 1h rather than 24h.  Only `query` and `langs` need to be provided as `task=search` is implied.
