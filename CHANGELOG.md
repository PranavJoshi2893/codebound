# Changelog

All notable changes to CodeBound will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com),
and this project adheres to [Semantic Versioning](https://semver.org).

## [v0.1.4-nightly.4] - 2026-11-08

### Added
- Propagate context creation-site IDs through actual call arguments, retaining possible/unknown inputs, parent relationships, and cancellation evidence.
- Browser graph zoom controls, cursor-centered wheel zoom, drag-to-pan, fit/reset, and keyboard navigation without external JavaScript dependencies.
- Context usage annotations in JSON, DOT, and browser call trees: parameters, creation and parent origins, timeouts, forwarding, cancellation-channel receives, and cancellation calls, with closure scopes and explicit unresolved origins.

### Changed
- Use a hybrid context view: usage inside function boxes, context IDs on call arrows, and creation/lifecycle markers showing starts and possible cancellation conditions.
- Keep polygonal connectors and combine repeated equivalent call edges, while keeping different context bindings distinct and preserving JSON findings.
- Organize call-tree loading, root selection, function descriptions, and traversal into focused methods with shared analysis state and simple return types.

### Fixed
- Let Graphviz reserve space for context arrow labels and nodes, avoiding crowded text from forced same-row layouts.
- Replace signature-only CHA callback targets with value-flow-based VTA analysis to avoid false self-calls such as `main → main` from `defer stop()`, while preserving genuine recursion.

## [v0.1.4-nightly.3] - 2026-11-07

### Added
- Interactive Charm menu for project creation and call-tree viewing.
- Call-tree analysis with architecture annotations, a local browser viewer, and DOT/JSON exports.
- Automatic executable entry-point discovery, an interactive root selector, positional project paths, and automatic viewer port selection.
- Regression tests and synthetic benchmarks for large graph exports and repeated analysis.

### Changed
- Share project validation and scaffolding between Cobra commands and the TUI.
- Cache the CHA graph per loaded analysis session and use iterative traversal with cached function descriptions.
- Stream DOT output without a duplicate graph representation or embedded DOT template.
- Document performance limits and pin installation instructions to this prerelease.

### Security
- Generate `.env` with owner-only permissions on Unix systems.
- Confine generated writes with `os.Root`, refuse overwrites, and require fresh API-module directories.
- Validate module identifiers and prevent module-path argument injection.
- Return module-generation failures to the CLI instead of ignoring them.
- Protect the local viewer against DNS rebinding and cross-site access; add security headers and HTTP timeouts.
- Bound Graphviz rendering time, input, output, and diagnostic sizes.
- Bind generated HTTP servers to localhost with request timeouts; public deployments require an HTTPS reverse proxy.
- Remove wildcard CORS from newly generated routers.

## [v0.1.4-nightly.2] - 2026-09-25

### Added
- Standard response templates for router, handler, and service layers.

### Changed
- Updated `create.go` to integrate new templates and standard response generation.

## [v0.1.4-nightly.1] - 2026-09-25

### Changed
- Command changed from `init` to `create`.

## [v0.1.3-alpha] - 2026-09-23

### Changed

- Reset the pre-release version history after recreating the repository.
- Retracted previously published versions that should no longer be used:
    - v0.1.0-alpha
    - v0.1.1-alpha
    - v0.1.1
    - v0.1.2-alpha
- Established v0.1.3-alpha as the current development release.


## [v0.1.0-alpha] - 2026-09-23

### Added
- Initial release of CodeBound.
- Go project scaffolding with a pre-configured opinionated structure (`cmd`, `internal/config`, `internal/database`, `internal/router`, `internal/server`, `migrations`, `Makefile`).
- Support for custom Go module paths via `-m` / `--mod`.
- CLI-based project creation (`codebound create`).
