---
# t8q-tz6
title: Add WMA file support via minimal ASF parser
status: completed
type: feature
priority: normal
created_at: 2026-03-11T00:20:14Z
updated_at: 2026-03-11T00:21:47Z
sync:
    github:
        issue_number: "20"
        synced_at: "2026-03-11T00:45:59Z"
---

- [x] Create internal/scan/asf.go with ASF metadata reader
- [x] Create internal/scan/asf_test.go with unit tests
- [x] Add .wma to supportedExts in scan.go
- [x] Add ASF fallback in readTags
- [x] Update scan_test.go for .wma
- [x] Run lint and tests


## Summary of Changes

Added WMA file support via a minimal pure-Go ASF header parser. No new dependencies.
