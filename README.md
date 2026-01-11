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

You can override the `/httpapi` backend timeout and rate-limit window using `HTTPAPI_BACKEND_DURATION` (default: `3s`). This value is used for both the cache backend timeout and `min_duration`.

Additional mappings provided against `/httpapi/` so it will be a single root.
* `/httpapi/anime-titles.dat.gz` maps to https://anidb.net/api/anime-titles.dat.gz
* `/httpapi/anime-titles.xml.gz` maps to https://anidb.net/api/anime-titles.xml.gz
* `/httpapi/images/main/*` maps to https://cdn.anidb.net/images/main/
* `/httpapi/search` maps to the [search server](https://anisearch.outrance.pl/doc.html) though it's TTL is 1h rather than 24h.  Only `query` and `langs` need to be provided as `task=search` is implied.
