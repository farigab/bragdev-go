# Changelog

All notable changes to this project are documented in this file.

This project follows the "Keep a Changelog" format and Semantic Versioning.

Generated: 2026-04-20

## Unreleased - 2026-04-21

### Added (Unreleased)

- `internal/logger`: lightweight logging facade and simple structured logging helpers (`Infow`, `Errorw`, `Debugw`) with `Init()` to set level.
- `internal/httpresp`: JSON error helper `JSONError` for consistent API error responses.
- `internal/middleware`: `RequestLogger` middleware that logs requests, latency and recovers from panics.
- `internal/validation`: helpers to parse ISO dates and validate date ranges and repository lists.
- `internal/integration/close.go`: `closeBody` helper to centralize response body closing.
- New repository constructors and helpers (`NewPostgresUserRepo`, `NewPostgresRefreshTokenRepo`) and additional repository methods.
- Tooling and helper scripts: `.golangci.yml`, `lint.bat`, `run.bat`.

### Changed (Unreleased)

- Replaced usages of the standard `log` package with the new `internal/logger` across the codebase.
- `cmd/bragdoc/main.go`: initialize logger early, use logger for startup/errors, register `RequestLogger` middleware, and improve shutdown handling.
- `internal/integration/gemini.go`: rename fields to `apiURL`/`apiKey`, expose `NewGeminiClient(apiKey, apiURL, model)`, and use `closeBody` for safer response handling.
- `internal/handlers`: improved error handling and replaced plain-text responses with `httpresp.JSONError` in several endpoints; GitHub import handlers reorganized and split for clarity.
- `internal/security/jwt_service.go`: added convenience methods, safer startup behavior when `JWT_SECRET` is missing, and helper `ExtractUserLoginSafe`.
- Misc refactors and small API improvements across `usecase`, `repository`, `domain`, and `integration` packages.

### Misc (Unreleased)

- Added linter configuration and developer helpers; small Windows helper scripts.

## Added

- GitHub integration — Added GitHub OAuth flow and commit-fetching integration. (Commit: 097573c, 2026-04-20; Author: Gabriel Farias)
- Gemini AI integration — Added `GenerationConfig` (temperature, topP, topK, maxOutputTokens) and `WithGenerationConfig()` helper; client now sends `generationConfig` in requests and uses sensible defaults (temperature=0.4); HTTP client timeout increased to 30s. (File: internal/integration/gemini.go)
- Report response enhancements — Include `generated_at` timestamp and `report_type` fields in report responses for improved traceability. (Commit: 94f8eae, 2026-04-20; Author: Gabriel Farias)
- JWT refresh rotation — Implement refresh-token rotation in the authentication middleware to improve security. (Commit: 860f80d, 2026-04-20; Author: Gabriel Farias)

### Changed

- Achievements removal — Removed the achievements domain and repository; dropped the `achievements` table and related index from the initial database schema. (Commits: 70dd7fb, 7204997; 2026-04-20; Author: Gabriel Farias)

### Contributors

- Gabriel Farias — primary contributor for the changes listed above.
