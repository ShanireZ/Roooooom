# Portable development packages

Roooooom source is licensed under GPL-3.0-only; see [LICENSE](../LICENSE). Generated packages include that license and dependency notices. This is an experimental, unsigned development package, not a supported binary release. Platform support and minimum OS versions are recorded in [platforms.json](platforms.json). Checksums verify bytes, not trusted publisher identity.

Keep core and launcher together. On Windows open roooooom-launcher.exe; candidate Unix targets use roooooom-launcher. A generated package runs without Go, Node, Python, Docker or an external database server, but needs a modern browser and its OS dependencies. Build maintainers still require the pinned toolchain.

First use opens a private activation page, then guides project registration and explicit local enrollment. The shared skill is skills/roooooom/SKILL.md. A user must authorize a particular project's adoption before its agent changes a managed guide block. Installation does not install host settings, hooks, PATH entries, startup entries or global skills.

The Windows tray opens the board, reactivates the owner, exits the tray while retaining the service, or stops this product explicitly. Closing the browser does not stop the service. Headless fallback uses launcher --headless; --open opens the board, --activate opens a fresh private activation page, and --stop stops the product for the selected data root. Use --data-dir with an absolute private path for isolated tests.

Runtime state, credentials and outboxes belong in a separate private data root. A data root inside the installation is rejected. GitHub and DingTalk are disabled by default and require an explicit target/content confirmation. Notifications are not approval.

## Build and verify

```powershell
node scripts/build-web.mjs
node scripts/package.mjs --target windows-amd64 --out .local/packages/dev --skip-web
node scripts/verify-bundle.mjs --bundle .local/packages/dev/roooooom-windows-amd64
node scripts/verify-bundle.mjs --bundle .local/packages/dev/roooooom-windows-amd64 --self-test
```

Output must be new; do not overwrite an existing candidate. The package contains source/build fingerprints, UI inventory, same-version API/skill, GPL license and discovered third-party license texts. Dependencies retain their own terms; generated notices are an inventory, not a legal certification. Binary release publication is a separate operation from publishing source.

## Manual upgrade and recovery

1. Check the new package hashes, manifest, platform requirements and signing status. Keep the original package separately.
2. Stop the original product with launcher --stop --data-dir PRIVATE_ROOT and confirm its writer released. Do not terminate another program.
3. Use the original core's admin backup --data-dir PRIVATE_ROOT to create a consistent snapshot. Keep its identity and private data outside the package and Git.
4. Verify the application ID and schema migration. Schema 2 requires a forward migration; old binaries reject it. Launching a binary is not proof of data compatibility.
5. Unpack into a separate directory and launch with the same explicit data root. Verify identity, prior records, credentials and unresolved outbox entries. No old claim or completion silently replays.
6. A binary downgrade is safe only while data has not changed incompatibly. Otherwise restore the identified snapshot through the product's explicit restore procedure. Restore changes generation, invalidates old credentials/cursors/candidates and requires explicit identity recovery and integration re-confirmation.

SHA256SUMS covers payload files. manifest.json records exact build/source/UI inventory; the archive .sha256 additionally covers the whole ZIP. Private machine backups and provider credentials remain the user's responsibility.
