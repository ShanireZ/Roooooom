# Restore and clean-platform verification

This is a procedure, not a passed report. See the [design](../../DESIGN.md) and [evidence format](../../docs/evidence/acceptance-template.md).

Restore is a separate explicit owner/admin operation. Use a disposable private root and a consistent snapshot with its snapshot/service identity. Never test against an active user's database or copy credentials into a package.

Create known records, unread messages, pending questions, integration evidence and unresolved native outbox entries. Stop only this product, back up using its current tool and restore through the identified procedure. The generation changes: old cookies, tokens, cursors and candidates must fail. Recover the same participant explicitly; resume or hand off responsibility separately. External bindings remain disabled until confirmed and old unknown writes do not blindly replay.

Exercise schema 1 to 2 migration with existing history and integrity/foreign-key checks. Older binaries reject schema 2. Changing only the executable is not a rollback once data is incompatible; use the snapshot procedure.

For A36, use a clean OS/user profile or VM without Go, Node, Python, Docker or a database server. Run the actual complete package, activate and enroll a synthetic project, close the browser, reopen through the tray/launcher, stop/restart and perform a recoverable upgrade. Record OS/build, architecture, browser, system dependencies and signing prompts. Do not bypass OS/browser warnings.

A development-machine subprocess smoke test, static cross-platform checksum or container cannot replace clean-machine/default-browser/device evidence. No clean environment means unverified, and platform supported stays false. Retain precise private observations and sanitized evidence references for A16/A17/A30/A36.
