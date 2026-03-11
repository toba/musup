---
# m3f-ejq
title: Add Windows exe to GitHub release
status: completed
type: task
priority: normal
created_at: 2026-03-11T01:24:16Z
updated_at: 2026-03-11T01:25:31Z
sync:
    github:
        issue_number: "22"
        synced_at: "2026-03-11T01:32:09Z"
---

Build and publish a Windows executable (.exe) as part of the GitHub release process. This likely involves adding a cross-compilation step (GOOS=windows GOARCH=amd64) to the release workflow or Makefile.

## Summary of Changes

No changes needed. The existing configuration already builds and publishes Windows executables:

- `.goreleaser.yaml` includes `windows` under `goos` (line 12) and packages them as `.zip` archives (line 29)
- `.github/workflows/release.yml` has an `update-scoop` job (line 83) that publishes a Scoop manifest for both amd64 and arm64 Windows builds
- Both `musup_windows_amd64.zip` and `musup_windows_arm64.zip` are already part of every release
