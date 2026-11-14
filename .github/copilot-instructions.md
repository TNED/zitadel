# GitHub Copilot Instructions for ZITADEL

You are working in the ZITADEL repository. This codebase is a pure headless Go backend.

**CRITICAL**: Read `AGENTS.md` in the root first, then the nearest scoped `AGENTS.md` for changed files.

## Key References
- `AGENTS.md`: Root architecture map and global commands.
- `internal/AGENTS.md`: Backend domain and storage SPI boundaries.
- `proto/AGENTS.md`: API schema and generation guidance.
- `deploy/compose/AGENTS.md`: Docker Compose deployment invariants, file conventions, and rejected alternatives.

## Behavior
- Before Go-related work, inspect `go.mod` for required Go version/toolchain.
- Use Makefile targets (`make build`, `make test`, `make lint`).
- Respect terminology defined in the `Technical Glossary` section of `AGENTS.md`.
- For user-facing wording updates across docs/API descriptions, follow the canonical table in `TERMINOLOGY.md` (issue #5888).
- For deployment-related work, consult the directory-specific agents files (e.g., `deploy/compose/AGENTS.md`) for invariants, file conventions, and rejected alternatives.
