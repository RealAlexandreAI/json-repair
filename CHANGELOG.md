# Changelog

## Unreleased

## v0.0.17

- Fix lint: remove unused `isSmartQuote`, fix ineffectual assignment.
- Upgrade CI: goreleaser v2, actions v6, Go 1.23.

## v0.0.16

### New Features

- Smart/curly quote support: `\u201c`, `\u201d`, `\u201e`, `\u2018`, `\u2019`, `\uff02`, `\uff07` (Chinese LLM full-width quotes).
- Full-width punctuation normalization: `｛｝［］：，；` → ASCII equivalents.
- Code fence handling: `` ```json ... ``` `` blocks stripped and parsed.
- Comment stripping: `//`, `/* */`, `#` comments removed from input.
- Multiple top-level JSON values collected into an array.
- Duplicate key deduplication: non-comma-separated duplicates split into separate objects.
- Code fence within string values: `` ```json ... ``` `` inside strings parsed as JSON.

### Bug Fixes

- Fixed infinite recursion on Unicode number bytes (issue #23).
- Fixed `stripComments` breaking strings with unescaped quotes (Issue #18 regression).
- Fixed `normalizePunctuation` corrupting invalid UTF-8 bytes.
- Fixed smart quote parsing skipping characters with double index increment.

### Improvements

- Ported upstream Python `json_repair` v0.61.0 features to Go.
- 80 test cases (up from 66), all passing.

## v0.0.15

- Fix version synchronization between CLI and Git tag using ldflags.

## v0.0.8

- More test cases.
- Sync upstream.

## v0.0.7

- Enhance README
- Add golangci-lint

## v0.0.6

- Use bytes.IndexByte instead of Contains
- fix upstream #29
- Split MustRepairJSON

## v0.0.5

### Improvements

- Tuning goreleaser, remove draft
- UT action
- Change minimal Go version

## v0.0.4

### New Features

- Add Homebrew tap.

## v0.0.3

### New Features

- Introduced `jsonrepair` as a command-line interface for easier use.

### Improvements

- Initialized `goreleaser` to streamline the release process.

### Bug Fixes

- Resolved an issue reported in upstream repository (#26).

## v0.0.2

### New Features

- Implemented `markerRecord` to enhance the tracking of nested data structures.

### Bug Fixes

- Addressed a bug identified in the upstream repository (#24).

### Deprecations

- (This section is left empty as there are no items to be removed in this release.)

## v0.0.1

### Initial Release

- Launched the initial version of `json-repair`, a tool designed to fix malformed JSON files.

### Deprecations

- (This section is left empty as there are no items to be removed in this initial release.)
