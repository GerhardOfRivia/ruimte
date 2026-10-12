# Aarde: Agent Instructions

## Scope and working approach

These instructions apply throughout this repository unless a more specific
instruction file applies. Implement the requested change with the smallest
coherent diff; do not turn a focused task into an architectural rewrite.

Before editing, inspect `git status --short --branch`, the relevant source and
tests, and the applicable guides under `docs/`. Work on the current branch and
preserve unrelated local changes. Do not reset, clean, switch branches, commit,
push, or create release tags unless explicitly requested. Never run `make release`
as a validation step: it creates and pushes a tag.

Use the active checkout to verify paths, commands, dependencies, and behavior.
When code and documentation disagree, investigate and report the discrepancy;
do not silently change a public contract just to match an assumption. Update
this file when an intentional architectural change makes its guidance stale.

## Project intent and architecture

Aarde catalogs local GeoTIFF and NITF imagery and provides spatial discovery and
native-image viewing. One Go application supplies the CLI, HTTP API, and embedded
React/TypeScript UI. PostgreSQL/PostGIS stores catalog data; GDAL command-line
tools inspect and render imagery. The UI uses OpenLayers and Material UI.

Preserve the existing package boundaries:

| Location | Responsibility |
| --- | --- |
| `cmd/aarde/main.go` | Process entry point and injected build version |
| `internal/cli` | Parsing, help, exit status, signals, and command orchestration |
| `internal/config` | Environment configuration and validation |
| `internal/catalog` | Shared catalog models and operations |
| `internal/database` | Database access and parameterized SQL |
| `migrations/` | Versioned SQL migrations |
| `internal/importer` | File discovery and import orchestration |
| `internal/raster` | GDAL integration, raster inspection, and viewer rendering |
| `internal/geo` | Bounded geometry validation |
| `internal/api` | HTTP contracts, request validation, DTOs, and OpenAPI |
| `internal/server` | HTTP server, middleware, embedded assets, and shutdown |
| `web/` | Frontend source, tests, and asset build |

Keep CLI and HTTP operations on the shared catalog service. Do not duplicate
business rules in handlers, move SQL into the CLI, or make domain packages import
`internal/cli`. Keep `main.go` thin; process exit belongs at that boundary.

Prefer existing dependencies and patterns. Do not introduce Cobra, Go GDAL
bindings, a Python application backend, queues, plugin systems, extra services,
or another frontend framework without an explicitly scoped requirement. A small
feature should not create a new deployment dependency.

Reference: [Development](docs/development.md).

## Build and validation

Use the versions declared by the checkout, lockfiles, Dockerfile, and CI.
The documented local baseline is Go 1.25+, Node 22.12+ with npm, GDAL 3.6+, and
PostgreSQL/PostGIS. GDAL-dependent tests need the relevant utilities and codecs;
some fixture generation also needs Python GDAL. The isolated integration setup
is the preferred way to obtain the complete test environment.

Run from the repository root:

```sh
# Install exact frontend dependencies, check contracts, and build embedded assets.
(cd web && npm ci && npm test && npm run build)

# Format changed Go files with gofmt before these checks.
go test ./...
go vet ./...
go test -race ./...
make build
git diff --check
```

Build the frontend **before** the full Go suite or a production binary. The
embedded placeholder allows compilation without a UI build, but does not supply
the frontend and documentation assets required by the full tests. `make build`
builds the Go binary; it does not run the frontend build. `make web` is the
Docker-based frontend build target.

For database, spatial, import, GDAL, or rendering changes, run:

```sh
make integration
```

Alternatively, use `AARDE_TEST_DATABASE_URL` with a dedicated disposable PostGIS
database and run `go test -race -count=1 ./...` after building frontend assets.
Never use a production database for tests. Distinguish `AARDE_DATABASE_URL`
(application configuration) from `AARDE_TEST_DATABASE_URL` (test configuration).

For viewer changes, also run the browser suite when Playwright and Chromium are
available:

```sh
(cd web && npm run test:browser)
```

Follow the existing `PLAYWRIGHT_MODULE` setup rather than adding dependencies
solely to accommodate the agent environment. Do not invent `lint`, `typecheck`,
or other npm scripts; inspect `web/package.json`. Its build runs TypeScript and
OpenAPI checks.

Run focused tests while iterating, then the relevant broader checks. Report
missing tools, skipped integration coverage, and failing commands accurately.
A passing suite with skipped GDAL/PostGIS tests is not full integration validation.
Documentation-only changes need link, command, and diff review rather than an
unrelated application rebuild; state when runtime tests were not run.

References: [Makefile](Makefile), [Frontend scripts](web/package.json),
[Testing](docs/development.md#testing).

## CLI compatibility

Preserve `serve`, `import`, `inspect`, `search`, `remove`, and `version`, their
existing options, and established output formats unless the task changes them.

- Keep standard-library flag parsing and support flags before or after positional
  arguments, including `--` for paths beginning with a dash.
- Validate syntax before loading configuration or opening the database. Help,
  version, and no-argument help must work without valid application configuration.
- Use the injected input/output streams. Preserve help, results, progress, and
  prompts on stdout; errors and diagnostic logs belong on stderr.
- Preserve exit codes: `0` for success and cancelled removal, `1` for operational
  errors, and `2` for invalid usage. Return errors/status rather than calling
  `os.Exit` or fatal logging from reusable code.
- Propagate cancellation through discovery, hashing, GDAL, database access, and
  server shutdown. Keep confirmation and cancellation testable without sending
  signals to the test process.

Do not casually change the existing logger restoration or synchronization used
for configured CLI invocations. Add regression tests for parsing, output,
configuration-free commands, and cancellation when touching this boundary.

References: [CLI guide](docs/cli.md), [CLI implementation](internal/cli/cli.go).

## Catalog and source preservation

The physical raster is authoritative. Do not modify, relocate, delete, or write
sidecars beside source imagery. Preserve read-only mounts and delivery support
files. Catalog removal deletes database records, never source assets.

Keep one catalog record per physical file, including multi-image NITFs. Preserve
catalog-scoped identity and checksum uniqueness. A checksum covers the complete
physical raster, not its sidecars. A renamed duplicate retains the existing
record; a conflicting ID is not permission to overwrite it. Adding metadata or
changing import overrides must not silently refresh duplicate records.

Dry runs must not migrate, write records, modify sources, or create persistent
derivatives. Preserve the difference between offline validation and read-only
database-assisted checks. Continue through individual bad files during directory
imports, report failures, and retain a nonzero final status when appropriate.

Use new migration files for schema evolution rather than rewriting applied
migrations. Preserve transaction/advisory-lock behavior and legacy rows. Do not
trigger migrations from read paths or dry runs. Include migration, uniqueness,
and concurrent-import tests for changes affecting persistence.

References: [Import and identity contracts](docs/cli.md),
[Migrations](docs/configuration.md#database-migrations-and-backups).

## Geospatial and NITF correctness

Keep source CRS information distinct from catalog footprints and viewer
coordinates. Catalog footprints are valid WGS84 `MultiPolygon` geometries; do
not substitute a bounding box for actual coverage or spatial intersection.
Test longitude/latitude order, nonfinite coordinates, invalid topology, holes,
disjoint coverage, and boundary cases when changing geometry behavior.

NITF image segments are not raster bands. Preserve explicit zero-based
`NITF_IM:<index>:<path>` selection, complete segment enumeration, and segment
ordering. Do not silently read only image zero or discard unsupported segments.
File coverage is the union of valid primary-imagery footprints; cloud auxiliaries
follow their documented exception and are excluded from that union.

Do not fabricate affine transforms, geographic placement, acquisition times, or
footprints from filenames, image dimensions, or loosely associated TRE corners.
Preserve the explicit limitations for RPC-only imagery and unsupported
antimeridian geometry. Approximate GCP footprints and viewer registration are
not orthorectification or survey-grade accuracy.

Use the existing GDAL-based path and bounded specialized readers; do not build a
general-purpose NITF parser as an incidental refactor. Keep unsupported codecs,
malformed containers, and absent georeferencing distinguishable and actionable.

Reference: [NITF support](docs/nitf.md).

## Metadata and cloud semantics

Unknown values remain unknown: `NULL` cloud cover is not clear sky, and import
or filesystem timestamps are not acquisition timestamps. Preserve original GDAL
metadata and put derived annotations under the reserved `_aarde` namespace.
Retain provenance, units, scope, ambiguity, and diagnostics.

Cloud cover is a percentage from 0 through 100. Preserve the documented source
precedence: explicit import override, valid matched XML, embedded GDAL values,
then the applicable associated TRE fallback. Maxar XML `CLOUDCOVER` is a 0–1
fraction; PIAIMC `CLOUDCVR` is a percentage with `999` meaning unknown. Do not
introduce magnitude-based unit guesses. Keep file-level and segment values
separate, and do not average conflicting segment values into a scene value.

`--cloud-cover-lt` means strictly less than; exclude unknown values when this
filter is supplied. Preserve explicit zero and test threshold boundaries. Do
not conflate strict and inclusive filters in the API or UI.

NCDRD recognition is evidence-based, not conformance certification. Vendor text,
segment count, or a cloud percentage cannot establish a cloud layer. Preserve
supported spatial cloud representations and their registration; distinguish
absent, unsupported, failed, and unregistered data. Keep categorical cloud grids
categorical, including nearest-neighbor sampling and reserved-value semantics.

Recognizing XML or TRE metadata does not establish complete Basic Level 1B
product-delivery support. Preserve accompanying support files and do not claim
new delivery formats or standards variants without implementation and tests.

References: [Metadata](docs/metadata.md),
[Commercial NITF](docs/commercial-nitf.md), [Filters](docs/cli.md#search).

## Native viewer and frontend

Preserve the dedicated `/image-viewer?catalog=...&image=...` route. These values
identify catalog records, not filesystem paths. Keep automatic scene-layer
loading, independent visibility/opacity, pan/zoom, per-layer failures, and
supported cloud overlays.

**Do not introduce imagery tiles, tile servers, COG conversion, persistent
pyramids, preparation jobs, or polling workflows.** Bounded temporary
whole-segment renders are allowed; source modification and persistent imagery
conversion are not. The catalog's configured basemap is separate from this
imagery-rendering constraint.

Pan, zoom, visibility, and opacity changes must not fetch new imagery. Preserve
explicit resolution replacement and bounded scene/layer budgets rather than
silently omitting segments. Abort stale requests and release decoded images,
map resources, listeners, and temporary storage on replacement or teardown.

Keep ordinary catalog reads on stored metadata; do not reopen rasters to serve
search results. Preserve explicit search behavior and responsive loading/error
states. Use existing React, TypeScript, Material UI, and OpenLayers patterns;
render metadata as escaped text. Keep offline behavior and avoid new mandatory
CDN or external-service dependencies.

Reference: [Image Viewer](docs/image-viewer.md).

## Security and API contracts

Preserve the read-only HTTP API and existing bearer-token middleware. Anonymous
catalog access does not authorize viewer content. Keep tokens out of URLs,
logs, fixtures, commits, and generated documentation. Do not add web imports,
uploads, destructive routes, permissive CORS, or a new identity system as
incidental feature work.

Resolve viewer sources through catalog identity, then enforce local-file,
symlink, source-root, and revision checks. Do not accept arbitrary paths,
remote URLs, or GDAL selectors from an HTTP client. Keep cache hits subject to
authorization and source validation.

Treat raster metadata and sidecars as untrusted input. Invoke subprocesses with
`exec.CommandContext` and separate arguments, never shell-interpolated strings.
Bound subprocess output, XML/JSON parsing, geometry, temporary storage, and
concurrency. Keep GDAL PAM writes disabled and clean temporary files on all
exit paths. Do not remove limits merely to make a large fixture pass.

Update `internal/api/openapi.json` alongside endpoint, query, DTO, validation,
or error changes. Keep handlers, frontend types, contract tests, and documentation
consistent. Do not introduce breaking fields or identity changes without
explicitly addressing compatibility.

References: [Authentication and configuration](docs/configuration.md),
[Viewer security](docs/image-viewer.md#rendering-security-and-resource-configuration),
[OpenAPI](internal/api/openapi.json).

## Tests, review, and completion

Prefer small deterministic, locally generated, unclassified fixtures. Do not
commit customer imagery, credentials, large rasters, build output, or dependency
directories. Use real PostGIS for spatial-correctness tests rather than mocks
that only compare SQL strings.

Add a regression test for each bug fix. Exercise negative paths as well as the
happy path: malformed input, unknown metadata, duplicate IDs/checksums, missing
files/codecs, cancellation, auth failures, and exhausted resource limits as
relevant to the change. Never weaken an assertion or remove a guardrail just to
obtain a green test run.

For viewer changes, follow the documented manual smoke test and browser checks;
verify actual placement/composited pixels, not only manifest shape. Keep good
layers usable when another layer fails.

Review the final diff for unrelated edits, generated artifacts, secrets, public
contract drift, and stale documentation. Update the relevant guide and
`docs/README.md` when adding documentation. Format only the code involved in the
change rather than creating repository-wide formatting noise.

Conclude each task with the change summary, tests actually run and their
results, and any remaining risks or unverified behavior. Distinguish confirmed
behavior from assumptions. Do not claim deployment, integration validation,
standards conformance, or test success that was not demonstrated.
