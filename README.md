# Roooooom

[![License: GPL-3.0-only](https://img.shields.io/badge/license-GPL--3.0--only-blue.svg?style=flat-square)](LICENSE)
![Status: experimental](https://img.shields.io/badge/status-experimental-orange.svg?style=flat-square)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg?style=flat-square&logo=go&logoColor=white)

[English](README.md) | [Simplified Chinese](README.zh-CN.md) | [Traditional Chinese (TW)](README.zh-TW.md) | [Japanese](README.ja.md)

A local coordination room for AI coding agents that keep their own hosts and model connections.

Give a project a short request, follow shared progress, and answer the decisions that need you. Agents prepare the task, divide responsibilities, exchange messages, submit evidence, and integrate the result.

## What it does

- Guided project enrollment and a one-field request form.
- Project boards, discussions, named participants, and a focused attention page.
- Atomic preparation and assignment claims, explicit handoffs, and stale-session fences.
- An integration agent that reviews immutable deliveries and records exact heads and checks.
- Formal human decisions with versioned scope, protected owner requirements, and acceptance.
- Offline work and explicit reconciliation; browser writes are never replayed automatically.
- Optional GitHub Issues records and signed DingTalk Webhook notifications.

Roooooom does not start or wake agents, manage vendor CLI/IDE/desktop hosts, inject hooks, proxy model APIs, or run Git on an agent's behalf. Agents need their existing command/file tools and the shared [skill](skills/roooooom/SKILL.md). A stopped conversation must be resumed in its original host to process new messages.

## Status

Experimental source project. No downloadable binary release is published yet. Development-host Windows checks, local Chromium workflows, and Linux container tests are available; clean-machine, vendor-host, signing, and real-provider certification are incomplete. See [platform status](packaging/platforms.json) and [verification](docs/testing.md). Repository availability is not a support or security certification.

## Build and try

Build tools: Go 1.27.3, Node.js 24.22.1, and npm 12.2.1. Versions are pinned in the repository. For a Windows amd64 candidate:

```powershell
git clone https://github.com/ShanireZ/Roooooom.git
cd Roooooom
npm --prefix web ci
node scripts/build-web.mjs --no-install
node scripts/package.mjs --target windows-amd64 --out .local/packages/dev --skip-web
```

Keep the generated package together and open `.local/packages/dev/roooooom-windows-amd64/roooooom-launcher.exe`. The packaged application runs without Go, Node, Python, Docker, or a separate database server. A modern browser is required; platform minimum versions are not certified.

The first-use flow opens a private activation page, then guides you through selecting a project reference and explicitly enabling local enrollment. Copy the generated instructions into your existing agent conversation. The service does not scan the referenced repository or install host configuration. Other build targets and upgrade precautions are in [packaging](packaging/README.md).

## Daily workflow

1. Add a request in a project, or ask an already connected agent to register work.
2. An agent claims preparation, reads project authorities, and asks formal questions when needed.
3. It converts the request to one complete task and coordinates additional collaborators.
4. Workers deliver exact commits and checks; the integration agent reviews and integrates them.
5. Answer decisions or a ready human acceptance on the attention page. Completion retains its evidence.

The browser's main navigation has Projects and Needs my attention. Complete specifications, history, and technical management remain available on demand. Chat is discussion, not approval.

## Development and privacy

Run `npm --prefix web run verify` after installing dependencies and building the UI. It checks API types, TypeScript, Web unit tests, the embedded build, Go tests/vet, and Chromium workflows. Install the Playwright Chromium dependency for browser tests. Bundle-specific tests need an explicit `ROOOOOOM_TEST_BUNDLE`; otherwise those cases skip. See [contributing](CONTRIBUTING.md).

Runtime databases, credentials, activation files, local guides, plans, rehearsal evidence, and generated packages stay outside Git. Local historical development records are not part of the public initial history. The publication gate is `node scripts/check-publication.mjs`. See [security](SECURITY.md), [design](DESIGN.md), and the [documentation index](docs/index.md).

## License

GNU General Public License version 3 only (`GPL-3.0-only`); see [LICENSE](LICENSE). Third-party dependencies retain their own licenses and notices. This software is provided without warranty.
