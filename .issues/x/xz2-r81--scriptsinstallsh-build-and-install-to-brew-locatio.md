---
# xz2-r81
title: scripts/install.sh — build and install to brew location
status: completed
type: task
priority: normal
created_at: 2026-03-20T19:12:25Z
updated_at: 2026-03-20T19:14:07Z
---

Create a minimal install script that builds musup and replaces the brew-installed binary for local iteration.

## Summary of Changes

- Created `scripts/install.sh` — builds musup with `ver=dev` and installs over the brew Cellar binary
- Created `.zed/tasks.json` with an `install` task to run the script from Zed
