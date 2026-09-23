# Changelog

All notable changes to CodeBound will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0-alpha.1.html).

## [v0.1.0-alpha] - 2026-09-23

### Added
- Initial release of CodeBound.
- Go project scaffolding with a pre-configured opinionated structure (`cmd`, `internal/config`, `internal/database`, `internal/router`, `internal/server`, `migrations`, `Makefile`).
- Support for custom Go module paths via `-m` / `--mod`.
- CLI-based project initialization (`codebound init`).