# Contributing

Roooooom coordinates agents without taking over their hosts. Preserve that boundary in code, docs, and examples.

## Toolchain and checks

Use Go 1.27.3, Node.js 24.22.1, and npm 12.2.1. Do not install dependencies at a parent workspace root.

```powershell
npm --prefix web ci
node scripts/build-web.mjs --no-install
node web/node_modules/@playwright/test/cli.js install chromium
npm --prefix web run verify
node scripts/check-skill-contract.mjs
node scripts/check-publication.mjs
```

Web must be built before Go because the server embeds generated UI files. `api/openapi.yaml` is JSON-compatible YAML; regenerate TypeScript with `npm --prefix web run api:generate` after a contract change. Check the generated file with `api:check`. The skill contract builds the current CLI and checks the same machine help; it does not prove real hosts follow the skill.

For bundle tests, build a new directory using the packaging instructions, run `scripts/verify-bundle.mjs`, then explicitly set `ROOOOOOM_TEST_BUNDLE` to that native bundle. Run its artifact tests with `go test ./tests -run 'TestBundle|TestUpgrade|TestUnverifiedPlatform' -count=1 -v`. Without that variable these cases skip. Do not advertise them as passed.

Linux race tests use `go test -race ./... -count=1` in a matching Go environment. A Linux container does not certify a Linux desktop or Windows tray.

## Change discipline

- Keep changes scoped. Add meaningful regressions for protocol, authority, concurrency, and retry behavior; use a real browser for UI changes.
- Keep original requirements, immutable evidence, generation/version fences, and explicit ownership. Never make a failed gate pass by removing a protection.
- Do not add vendor-host hooks, login access, model credentials, background host control, or proxy adapters.
- Keep prose documentation in English. Only the three translated README files are language exceptions. Product UI strings and multilingual test fixtures are not documentation translations.
- Run the publication check on the exact staged content. `.gitignore` cannot remove sensitive data from existing commits; inspect every outgoing commit. The initial public branch intentionally excludes private local development history.
- Keep operating guides, private plans, credentials, local data, dumps, and test evidence ignored. Use synthetic fixtures. Never put a real secret in an Issue, patch, screenshot, or log.
- Keep build artifacts out of source control. Binary publication requires its own reviewed artifact, source, license, checksums, and platform evidence.

Contributions to project-owned material are under GPL-3.0-only. Preserve third-party notices and identify imported material. A pull request is a proposal, not permission to deploy, publish a release, or access live integration credentials.
