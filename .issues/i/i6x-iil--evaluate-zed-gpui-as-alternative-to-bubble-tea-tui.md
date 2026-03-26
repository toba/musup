---
# i6x-iil
title: Evaluate Zed GPUI as alternative to Bubble Tea TUI
status: completed
type: task
priority: normal
created_at: 2026-03-26T19:28:57Z
updated_at: 2026-03-26T19:33:44Z
sync:
    github:
        issue_number: "85"
        synced_at: "2026-03-26T20:55:20Z"
---

Investigate whether [Zed's GPUI framework](https://github.com/zed-industries/awesome-gpui) could replace the current Bubble Tea terminal UI with a native GPU-accelerated interface.

## Research

- [x] Review awesome-gpui resource list and existing GPUI projects
- [x] Assess GPUI maturity, documentation, and API stability
- [x] Evaluate feasibility of building musup's UI in GPUI (artist browser, discography pane, sync/scan views)
- [x] Compare developer experience: Rust + GPUI vs Go + Bubble Tea
- [x] Consider cross-platform support (macOS, Linux, Windows)
- [x] Identify migration cost and architectural impact (Go → Rust or Go + Rust FFI)

## Findings

### Framework Maturity & API Stability

- GPUI is **pre-1.0** with frequent breaking changes between versions. Zed targets 1.0 in Spring 2026.
- Licensed **Apache 2.0** (permissive), separate from Zed's GPL.
- Not yet a standalone crate — lives inside the Zed monorepo. You depend on it via git ref in Cargo.toml.
- [Glass-HQ](https://github.com/Glass-HQ/gpui) has forked GPUI into a standalone repo adding iOS support, but it's early.
- Documentation is thin: "the best way to learn is to read the Zed source code." The [gpui-book](https://github.com/searls/gpui-book) and [docs.rs/gpui](https://docs.rs/gpui) exist but are incomplete.
- The [gpui-component](https://github.com/longbridge/gpui-component) library by Longbridge provides 60+ ready-made components (buttons, tables, modals, date pickers, etc.) and is used in production (Longbridge Pro trading terminal).

### Ecosystem & Community Projects

The [awesome-gpui](https://github.com/zed-industries/awesome-gpui) list (719 stars) shows ~40 projects:
- **Developer tools**: Arbor (agentic coding), helix-gpui, pgui (Postgres), zedis (Redis), postman-gpui
- **Media**: hummingbird (music player), vleer (streaming player) — directly relevant to musup's domain
- **Productivity**: Loungy (launcher), nohrs (file explorer), reminder (Notion alternative)
- **Libraries**: gpui-component (60+ widgets), gpui-router, gpui-nav, gpui-tea (Elm architecture)

Most community projects are small/early-stage. Zed itself and Longbridge Pro are the only battle-tested production apps.

### Deployment: macOS

- **Excellent.** GPUI renders via Metal. Zed ships as a ~100 MB DMG (220 MB uncompressed binary).
- Standard .app bundle distribution — drag to Applications, or install via `brew install --cask`.
- macOS is the most polished platform. Supports both Intel and Apple Silicon.
- A musup GPUI app would ship as a single .app bundle or Homebrew cask, same as current Go binary but as a native GUI app.

### Deployment: Windows

- **Functional but youngest platform.** Windows support landed October 2025 with a DirectX 11 backend.
- Zed has a stable Windows release but platform-specific bugs are still being discovered.
- Smaller community knowledge base on Windows. The backend is newer and less battle-tested than Metal/Vulkan.
- Distribution would be via MSI/exe installer or Scoop/WinGet (similar to current musup).

### Deployment: Linux

- Renders via Vulkan. Second most mature after macOS.
- Standard binary distribution via package managers.

### Agentic Tractability

This is where GPUI presents **significant challenges** for AI-assisted development:

1. **Small training corpus.** GPUI code exists primarily in the Zed repo and ~40 small community projects. AI models have far less GPUI training data compared to established frameworks (React, SwiftUI, Bubble Tea). Claude and other models will produce lower-quality GPUI code with more hallucination.

2. **Rust complexity.** GPUI requires managing lifetimes, borrows, Entity/Context patterns, and GPU rendering concepts. Rust's ownership model makes AI-generated code more likely to fail compilation compared to Go or TypeScript.

3. **Unstable API.** Frequent breaking changes mean AI training data goes stale quickly. Code generated from examples even 6 months old may not compile.

4. **Documentation gaps.** AI agents that rely on documentation retrieval (RAG) have little to work with. The primary learning method is "read Zed source code" — a 500K+ line codebase.

5. **Ecosystem tooling.** There's a LobeHub GPUI development "skill" and create-gpui-app scaffolding, but nothing approaching the mature tooling around React, Flutter, or even Bubble Tea.

6. **Positive signal: gpui-component.** The Longbridge component library with 60+ widgets and real documentation would provide a higher-level abstraction that's more tractable for agents — building UIs from pre-made components rather than raw GPUI primitives.

7. **Positive signal: Elm architecture.** The gpui-tea library brings Elm-style architecture to GPUI, which is conceptually identical to Bubble Tea's model. This would reduce the paradigm shift.

### Compared to Current Stack (Go + Bubble Tea)

| Dimension | Go + Bubble Tea | Rust + GPUI |
|-----------|----------------|-------------|
| Language familiarity (us) | High | Would need to learn Rust |
| Agentic tractability | High — massive Go corpus, simple patterns | Low — small corpus, complex ownership |
| API stability | Stable (v2 released) | Pre-1.0, frequent breaking changes |
| Documentation | Excellent (Charm ecosystem) | Thin, "read the source" |
| macOS deployment | Single binary, Homebrew | .app bundle, Homebrew cask |
| Windows deployment | Single binary, Scoop | Installer/Scoop, DX11 backend (newest) |
| UI richness | Terminal-constrained | GPU-rendered, album art, rich typography |
| Binary size | ~10-20 MB | ~100-220 MB |
| Build time | Fast (seconds) | Slow (Rust compilation, minutes) |
| Portability | SSH, headless, any terminal | Desktop-only, needs GPU |

## Recommendation

**Not recommended at this time.** The migration cost is very high (full rewrite in a new language with an unstable framework), and the agentic tractability story is poor — AI coding assistants will struggle with GPUI's small corpus, complex Rust patterns, and unstable APIs. The Go + Bubble Tea stack is well-suited to musup's needs and highly tractable for agentic development.

**Revisit when:**
- GPUI reaches 1.0 with stable APIs (possibly late 2026)
- The corpus of GPUI code grows significantly (more production apps, better docs)
- musup needs capabilities that truly require a native GUI (album art display, rich media, drag-and-drop)

**Alternative to consider:** If a native GUI becomes desirable, evaluate **Tauri + web frontend** as a middle ground — it pairs a Rust backend with a web UI that AI agents handle extremely well, ships as a small native binary, and supports all platforms maturely.
