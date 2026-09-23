# Changelog

All notable changes to CodeBound will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0-alpha.1.html).

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
- CLI-based project initialization (`codebound init`).