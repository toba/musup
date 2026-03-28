---
# 46b-mkx
title: Hidden ^ command to download 7digital zip files
status: review
type: feature
priority: normal
created_at: 2026-03-27T23:59:59Z
updated_at: 2026-03-28T00:22:58Z
sync:
    github:
        issue_number: "88"
        synced_at: "2026-03-28T03:31:42Z"
---

Add a hidden TUI command bound to `^` (caret) that:

1. Opens a modal/text input prompting for a 7digital download ID (e.g. `844708572` from `https://us.7digital.com/download/844708572`)
2. On Enter, navigates to `https://us.7digital.com/download/<id>`
3. Validates the page/download exists
4. Finds and downloads all zip files linked on the page
5. Saves them to an appropriate location

## Tasks

- [x] Inspect 7digital download page to understand structure and zip file links
- [x] Add `^` keybinding to TUI
- [x] Create modal/text input for download ID entry (3 fields: ID, email, password)
- [x] Implement page fetching and zip link extraction
- [x] Implement zip file downloading
- [x] Wire it all together


## Protocol Notes

### Login
- `POST https://us.7digital.com/signin` with form body: `referrer=&email=<email>&password=<password>`
- Response: 302 + `Set-Cookie: session=<JWT>; secureSession=<JWT>` (HttpOnly, `.7digital.com`)

### Download Page
- `GET https://us.7digital.com/download/<orderId>` with cookies
- Parse HTML: `form[action="/download/release/<releaseId>"]` + `input[name=formatId]` value

### Download ZIP
- `GET https://us.7digital.com/download/release/<releaseId>?formatId=<formatId>` with cookies
- 302 → signed `media.geo.7digital.com` URL (server generates OAuth sig)
- 200 with `Content-Disposition: attachment; filename="Artist - Album.zip"`

### Credentials
- Store `7digital_user` and `7digital_pass` in DB `settings` table
- Prompt in TUI modal if not set
