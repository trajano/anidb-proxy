# AniDB proxy
This is a simple caching proxy for AniDB to prevent the rate limits by ensuring requests are cached.

## Endpoints

* `/api/anime-titles.dat.gz` maps to https://anidb.net/api/anime-titles.dat.gz
* `/api/anime-titles.xml.gz` maps to https://anidb.net/api/anime-titles.xml.gz
* `/httpapi` maps to http://api.anidb.net/httpapi

Additional mappings provided against `/httpapi/` so it will be a single root.
* `/httpapi/anime-titles.dat.gz` maps to https://anidb.net/api/anime-titles.dat.gz
* `/httpapi/anime-titles.xml.gz` maps to https://anidb.net/api/anime-titles.xml.gz
* `/httpapi/search` maps to the [search server](https://anisearch.outrance.pl/doc.html) though it's TTL is 1h rather than 24h 