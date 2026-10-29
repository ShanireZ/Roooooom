# Verification and remaining work

This document describes evidence scope, not a blanket certification. A successful development run proves only its recorded source, configuration, toolchain, fixtures and artifact. Private historical rehearsal logs are intentionally excluded from this repository and cannot be treated as witnesses for the public initial commit.

## Automated checks

See [contributing](../CONTRIBUTING.md) for the pinned toolchain and commands. The main verification runs API type consistency, TypeScript, Web unit tests, production embed, full Go tests/vet and real Chromium workflows against isolated Go services. Tests cover claim races, conversion uniqueness, version invalidation, scoped identities, retry/unknown results, text safety, setup recovery and optional fake-provider configuration.

Bundle checks use a freshly built native package. The verifier checks hashes and launches the actual binary in an isolated root with development runtimes removed from PATH. Its counterexample builds a real no-UI binary and requires the normal gate to reject it. Bundle tests skip unless ROOOOOOM_TEST_BUNDLE is explicitly set; skip is not pass. A package smoke test on a development machine is not A36 clean-platform evidence.

`node scripts/acceptance-summary.mjs --self-test` checks report-completeness counterexamples, not an actual host. `node scripts/check-skill-contract.mjs` checks authored instructions/CLI agreement and synthetic guide changes, not whether a vendor model follows those instructions. `node scripts/check-publication.mjs` audits the staged public snapshot; it does not remove secrets from previously published history or establish legal compliance.

## Evidence classes

| Class | What it proves |
| --- | --- |
| synthetic | Local protocol, fixtures, browser automation or fake providers |
| real-host | Actual tool calls and subsequent processing in an existing vendor host |
| real-external | Explicitly authorized writes and observations at the real provider |
| real-platform | Actual OS/user-profile/device observations |

Each result binds a clean commit, lockfiles, exact bundle/source fingerprint, environment, command, output and limitations. Missing, failed, skipped and unverified remain distinct. Receipt state alone cannot prove that a model read a full message or performed the reported checks.

## Remaining certification and product work

- Rehearse the current package with at least two different existing vendor hosts, including outage, ended/new session, stale epochs, handoff, restore and superseded decisions.
- Validate first use, browser closure, tray, restart and upgrade on a clean Windows user environment without development tools.
- Test other OS/architectures before changing their support flags; container checks are insufficient.
- With separately authorized credentials and destinations, verify real GitHub record reconciliation and DingTalk delivery/mobile behavior.
- Establish signed release provenance and a repeatable release procedure before offering binaries as supported downloads.

The [platform matrix](../packaging/platforms.json) remains the machine-readable support boundary. Public source publication does not complete these items or authorize adoption into another project.
