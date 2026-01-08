# AniDB proxy
This is a simple caching proxy for AniDB to prevent the rate limits by ensuring requests are cached.

## Endpoints

* `/api/anime-titles.dat.gz` maps to https://anidb.net/api/anime-titles.dat.gz
* `/api/anime-titles.xml.gz` maps to https://anidb.net/api/anime-titles.xml.gz
* `/httpapi` maps to http://api.anidb.net:9001/httpapi
* `/images/main/*` maps to https://cdn.anidb.net/images/main/

## httpapi handling

The `/httpapi` route applies two extra behaviors:

* Responses are delayed so each request takes at least 2 seconds.
* If the first bytes of the response (after gzip decoding) contain `<error code="500">`, the proxy
  rewrites the status to 500 and sets `Cache-Control: no-store` to avoid caching error responses.

Additional mappings provided against `/httpapi/` so it will be a single root.
* `/httpapi/anime-titles.dat.gz` maps to https://anidb.net/api/anime-titles.dat.gz
* `/httpapi/anime-titles.xml.gz` maps to https://anidb.net/api/anime-titles.xml.gz
* `/httpapi/images/main/*` maps to https://cdn.anidb.net/images/main/
* `/httpapi/search` maps to the [search server](https://anisearch.outrance.pl/doc.html) though it's TTL is 1h rather than 24h.  Only `query` and `langs` need to be provided as `task=search` is implied.
