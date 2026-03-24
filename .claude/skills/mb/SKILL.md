---
name: mb
description: >
  Expert knowledge of the MusicBrainz Web Service API (ws/2). Use when:
  (1) writing or modifying code that calls the MusicBrainz API,
  (2) constructing search queries, browse requests, or lookup requests,
  (3) choosing correct inc= parameters, type/status filters, or pagination strategy,
  (4) debugging MusicBrainz API errors or unexpected responses,
  (5) adding new MusicBrainz entity types or endpoints to the codebase,
  (6) working with files in internal/integration/musicbrainz/,
  (7) user mentions musicbrainz, mb, mbid, release-group, or MusicBrainz API.
---

# MusicBrainz API

Base: `https://musicbrainz.org/ws/2`

## Critical Rules

1. **Rate limit**: Max 1 request/second. Use `rate.NewLimiter(rate.Every(time.Second), 1)` and call `limiter.Wait(ctx)` before every request.
2. **User-Agent**: Required on every request. Format: `AppName/Version ( contact-url-or-email )`.
3. **JSON**: Set `Accept: application/json` header or add `fmt=json` query param.

## Three Request Types

### Lookup — single entity by MBID
```
GET /<entity>/<mbid>?inc=<INC>&fmt=json
```
Linked entities in lookups capped at 25. Use browse for more.

### Browse — linked entities by MBID
```
GET /<result-entity>?<linked-entity>=<mbid>&limit=100&offset=0&inc=<INC>&fmt=json
```
Pagination: `limit` max 100 (default 25). Page through with `offset`. For releases: offset by actual count returned (track cap ~500/page).

### Search — Lucene query
```
GET /<entity>?query=<lucene>&limit=25&offset=0&fmt=json
```
Field-specific: `artist:"Name"`. Boolean: `AND`, `OR`, `NOT`. Quoted phrases, wildcards (`*`), negation (`-field:value`).

## Key Patterns for This Codebase

**Artist search**: `GET /artist?query=artist:"<name>"&limit=5&fmt=json`
- Response: `{ artists: [{ id, name, sort-name, type, score, life-span, aliases, tags, ... }] }`
- Match by `score` (0–100). This project uses threshold of 90.

**Browse release-groups by artist**: `GET /release-group?artist=<mbid>&type=album&limit=100&offset=0&fmt=json`
- Response: `{ release-group-count, release-group-offset, release-groups: [{ id, title, primary-type, secondary-types, first-release-date, artist-credit, ... }] }`
- Page through all results incrementing offset by page size.
- Filter with `type=album`, `type=album|ep|single`, etc.

**Artist lookup with details**: `GET /artist/<mbid>?inc=aliases+tags&fmt=json`
- Returns full artist with life-span (begin/end/ended), type, country, area.

## Type & Status Filters

Primary types: `album`, `single`, `ep`, `broadcast`, `other`
Secondary types: `compilation`, `live`, `soundtrack`, `remix`, `demo`, `dj-mix`, `mixtape/street`, `interview`, `spokenword`, `audiobook`, `audio drama`, `field recording`
Status: `official`, `promotion`, `bootleg`, `pseudo-release`, `withdrawn`, `cancelled`

Pipe-separate multiples: `type=album|ep`. Combine: `type=live&status=bootleg`.

For release-groups by artist: `release-group-status=website-default` excludes promo/bootleg-only groups.

## Common inc= Values

- **Browse release-groups**: `artist-credits` (for artist names on each group)
- **Browse releases**: `artist-credits`, `labels`, `media`, `recordings`, `release-groups`
- **Lookup artist**: `release-groups`, `releases`, `recordings`, `works`, `aliases`, `tags`, `genres`
- **All entities**: `tags`, `genres`, `aliases`, `annotation`, relationship includes (`artist-rels`, `url-rels`, etc.)

Combine with `+`: `inc=artist-credits+tags+genres`

## JSON Response Shapes

Artist search result:
```json
{ "created": "...", "count": 5, "offset": 0, "artists": [{ "id": "<mbid>", "name": "...", "sort-name": "...", "type": "Person|Group", "score": 100, "country": "US", "life-span": { "begin": "1970", "end": null, "ended": false }, "aliases": [{ "name": "...", "sort-name": "...", "type": "...", "locale": "en" }], "tags": [{ "count": 5, "name": "rock" }] }] }
```

Release-group browse result:
```json
{ "release-group-count": 42, "release-group-offset": 0, "release-groups": [{ "id": "<mbid>", "title": "...", "primary-type": "Album", "secondary-types": [], "first-release-date": "2023-01-15", "disambiguation": "", "artist-credit": [{ "name": "...", "artist": { "id": "<mbid>", "name": "..." }, "joinphrase": "" }] }] }
```

## Cover Art Archive

Not part of ws/2. Separate service:
```
https://coverartarchive.org/release/<mbid>/front        # full size
https://coverartarchive.org/release/<mbid>/front-250    # 250px thumbnail
https://coverartarchive.org/release-group/<mbid>/front  # by release-group
```

## Full Reference

For complete details on all entity types, all search fields, the browse matrix, all inc= values, data submission endpoints, and non-MBID lookups, read [references/api_reference.md](references/api_reference.md).
