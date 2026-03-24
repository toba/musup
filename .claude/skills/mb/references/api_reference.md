# MusicBrainz API Reference

Base URL: `https://musicbrainz.org/ws/2`

## Table of Contents

- [Authentication & Headers](#authentication--headers)
- [Rate Limiting](#rate-limiting)
- [Response Format](#response-format)
- [Entities](#entities)
- [Lookup Requests](#lookup-requests)
- [Browse Requests](#browse-requests)
- [Search Requests](#search-requests)
- [inc= Parameter Reference](#inc-parameter-reference)
- [Release Type & Status Filtering](#release-type--status-filtering)
- [Non-MBID Lookups](#non-mbid-lookups)
- [Data Submission](#data-submission)
- [Cover Art Archive](#cover-art-archive)

## Authentication & Headers

**User-Agent** (required): Every request must include a meaningful User-Agent header. Format: `AppName/Version ( contact-url-or-email )`. Requests without a proper User-Agent may be blocked.

**Accept header**: Set `Accept: application/json` for JSON responses, or append `&fmt=json` to the query string.

**Authentication**: Required only for:
- Data submission (POST/PUT/DELETE)
- User-specific data (`user-tags`, `user-ratings`, `user-genres`)

Methods: OAuth2 or Digest auth over HTTPS using musicbrainz.org credentials.

## Rate Limiting

**Hard limit: 1 request per second.** Exceeding this may result in IP blocking. Use `golang.org/x/time/rate` with `rate.NewLimiter(rate.Every(time.Second), 1)`.

## Response Format

- Default: XML (MMD 2.0 schema)
- JSON: via `Accept: application/json` header or `fmt=json` query parameter
- All responses include entity-specific data with MBIDs as UUIDs

## Entities

13 core entity types: `artist`, `release`, `release-group`, `recording`, `work`, `label`, `area`, `event`, `genre`, `instrument`, `place`, `series`, `url`

Non-core: `rating`, `tag`, `collection`

Special lookup resources: `discid`, `isrc`, `iswc`

## Lookup Requests

```
GET /<ENTITY_TYPE>/<MBID>?inc=<INC>
```

Returns a single entity by its MBID. Use `inc=` to request additional linked data.

**Note**: Linked entities in lookups are always limited to 25. Use browse requests for more.

Examples:
```
GET /artist/5b11f4ce-a62d-471e-81fc-a69a8278c7da?inc=release-groups&fmt=json
GET /release-group/abc123?inc=releases+artist-credits&fmt=json
```

## Browse Requests

```
GET /<RESULT_ENTITY>?<LINKED_ENTITY>=<MBID>&limit=<N>&offset=<N>&inc=<INC>
```

Fetch all entities of one type linked to another entity's MBID. NOT a search — requires the linked entity's MBID.

### Pagination

| Parameter | Default | Maximum |
|-----------|---------|---------|
| `limit`   | 25      | 100     |
| `offset`  | 0       | —       |

**Release pagination caveat**: Results capped so total tracks per page ≤ ~500. Increment offset by actual count returned, not the limit value.

### Browse Linked Entity Matrix

| Result Entity | Browsable By |
|---------------|-------------|
| area | collection |
| artist | area, collection, recording, release, release-group, work |
| collection | area, artist, editor, event, label, place, recording, release, release-group, work |
| event | area, artist, collection, event, place |
| genre | collection |
| instrument | collection |
| label | area, collection, release |
| place | area, collection |
| recording | artist, collection, release, work |
| release | area, artist, collection, label, track, track_artist, recording, release-group |
| release-group | artist, collection, release |
| series | collection |
| work | artist, collection |

`track_artist` — artist appears in track credits but not the release-level artist credit.

### Browse Examples

```
# All release-groups by an artist (albums only)
GET /release-group?artist=<MBID>&type=album&limit=100&offset=0&fmt=json

# All releases on a label
GET /release?label=<MBID>&limit=100&offset=0&fmt=json

# All recordings by an artist
GET /recording?artist=<MBID>&limit=100&offset=0&fmt=json
```

## Search Requests

```
GET /<ENTITY_TYPE>?query=<LUCENE_QUERY>&limit=<N>&offset=<N>&fmt=json
```

15 searchable entity types: `annotation`, `area`, `artist`, `cdstub`, `event`, `instrument`, `label`, `place`, `recording`, `release`, `release-group`, `series`, `tag`, `work`, `url`

Pagination: `limit` (1–100, default 25), `offset`.

### Lucene Query Syntax

- Field-specific: `artist:"Miles Davis"`
- Boolean: `artist:"Miles" AND country:US`
- Negation: `-field:value`
- Null check: `-field:*`
- Quoted phrases: `"exact phrase"`
- Wildcard: `artist:mile*`
- Escape special chars: `+ - && || ! ( ) { } [ ] ^ " ~ * ? : \ /`

### Artist Search Fields

| Field | Description |
|-------|-------------|
| `artist` | Artist name (default search field along with alias and sort name) |
| `arid` | Artist MBID |
| `alias` | Aliases |
| `type` | `person`, `group`, `orchestra`, `choir`, `character`, `other` |
| `country` | ISO 3166-1 two-letter code |
| `gender` | `male`, `female`, `other`, `not applicable` |
| `area` | Artist area name |
| `begin` / `end` | Life span dates (YYYY-MM-DD) |
| `ended` | Boolean — has ended |
| `tag` | Folksonomy tag |
| `ipi` / `isni` | Identifiers |
| `comment` | Disambiguation comment |

### Release Search Fields

| Field | Description |
|-------|-------------|
| `release` | Release title |
| `reid` | Release MBID |
| `arid` | Artist MBID |
| `artist` | Artist name |
| `alias` | Artist alias |
| `status` | Release status |
| `format` | Media format (CD, Vinyl, Digital Media, etc.) |
| `country` | Release country |
| `barcode` | Barcode |
| `catno` | Catalog number |
| `label` | Label name |
| `year` | Release year |
| `asin` | Amazon ASIN |
| `type` | Release group primary type |
| `comment` | Disambiguation |
| `tag` | Tag |

### Release-Group Search Fields

| Field | Description |
|-------|-------------|
| `releasegroup` | Title |
| `rgid` | Release-group MBID |
| `arid` | Artist MBID |
| `artist` | Artist name |
| `alias` | Artist alias |
| `primarytype` | Primary type (album, single, ep, etc.) |
| `secondarytype` | Secondary type (compilation, live, soundtrack, etc.) |
| `releases` | Number of releases in group |
| `tag` | Tag |
| `comment` | Disambiguation |

### Recording Search Fields

| Field | Description |
|-------|-------------|
| `recording` | Title |
| `rid` | Recording MBID |
| `arid` | Artist MBID |
| `artist` | Artist name |
| `alias` | Artist alias |
| `isrc` | ISRC code |
| `dur` | Duration in milliseconds |
| `firstreleasedate` | First release date |
| `status` | Release status |
| `format` | Media format |
| `country` | Release country |
| `tag` | Tag |
| `video` | Boolean — is video |

## inc= Parameter Reference

Combine multiple values with `+`: `inc=recordings+artist-credits+tags`

### Lookup Subqueries (linked entities)

| Entity | Valid inc= subqueries |
|--------|----------------------|
| artist | recordings, releases, release-groups, works |
| label | releases |
| recording | releases, release-groups |
| release | collections, labels, recordings, release-groups |
| release-group | releases |

### Subquery Modifiers

- `discids` — disc IDs for media
- `media` — media info with track counts
- `isrcs` — ISRC codes for recordings
- `artist-credits` — full artist credits
- `various-artists` — (artist lookups only) include only releases where artist appears on tracks but not release credit

### Browse inc= (restricted subset)

| Entity | Available inc= |
|--------|----------------|
| area, artist, event, instrument, label, place, series, work | aliases |
| recording | artist-credits, isrcs |
| release | artist-credits, labels, recordings, release-groups, media, discids, isrcs |
| release-group | artist-credits |
| url | relationship includes only |

All browse entities also support: `annotation`, `tags`, `user-tags`, `genres`, `user-genres`, and all relationship includes.

### Universal Includes (lookup and browse)

`aliases`, `annotation`, `tags`, `ratings`, `genres`, `user-tags`, `user-ratings`, `user-genres`

### Relationship Includes

Available for all entity types (except genre):

`area-rels`, `artist-rels`, `event-rels`, `genre-rels`, `instrument-rels`, `label-rels`, `place-rels`, `recording-rels`, `release-rels`, `release-group-rels`, `series-rels`, `url-rels`, `work-rels`

Nested (release lookups only): `recording-level-rels`, `release-group-level-rels`, `work-level-rels`

## Release Type & Status Filtering

Use `type=` and `status=` query parameters on browse requests.

### Primary Types

`album`, `single`, `ep`, `broadcast`, `other`

### Secondary Types

`audio drama`, `audiobook`, `compilation`, `demo`, `dj-mix`, `field recording`, `interview`, `live`, `mixtape/street`, `remix`, `soundtrack`, `spokenword`

### Status Values

`official`, `promotion`, `bootleg`, `pseudo-release`, `withdrawn`, `cancelled`

### Combining Filters

```
# Live bootleg releases by artist
GET /release?artist=<MBID>&status=bootleg&type=live

# Albums and EPs by artist (pipe-separated)
GET /release-group?artist=<MBID>&type=album|ep
```

### release-group-status Parameter

For browsing release-groups by artist:
- `release-group-status=website-default` — exclude groups with only promo/bootleg/pseudo-release status
- `release-group-status=all` — include everything

## Non-MBID Lookups

```
# By Disc ID
GET /discid/<discid>?inc=<INC>&toc=<TOC>

# By ISRC (returns recordings)
GET /isrc/<isrc>?inc=<INC>

# By ISWC (returns works)
GET /iswc/<iswc>?inc=<INC>

# By URL text (up to 100 resource= params)
GET /url?resource=<URL>
```

## Data Submission

All submissions require `client=<app-version>` in URL and authentication.

Content-Type: `application/xml; charset=utf-8`

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/tag?client=<ID>` | POST | Submit tags/genres (XML with `<user-tag>` elements, optional `vote` attribute) |
| `/rating?client=<ID>` | POST | Submit ratings (XML with `<user-rating>`, values 1–100) |
| `/collection/<gid>/<entity-type>/<mbid;mbid;...>?client=<ID>` | PUT/DELETE | Add/remove collection members (semicolon-separated, ≤400, URI ≤16KB) |
| `/release/?client=<ID>` | POST | Submit barcodes |
| `/recording/?client=<ID>` | POST | Submit ISRCs |

## Cover Art Archive

Cover art is served from the Cover Art Archive (CAA), not the MusicBrainz API itself.

```
# Front cover by release MBID
https://coverartarchive.org/release/<MBID>/front
# Also accepts: front-250, front-500, front-1200

# By release-group MBID (uses most appropriate release)
https://coverartarchive.org/release-group/<MBID>/front

# All images metadata
https://coverartarchive.org/release/<MBID>/
```

Returns JSON with image list including `thumbnails` (250/500/1200), `types`, `front`/`back` booleans, and `image` URL.
