# Security

Roooooom is experimental and has no supported production release. It is intended for one trusted OS user on one machine, bound to IPv4 loopback. Do not expose it through a LAN listener, reverse proxy, or public tunnel.

The owner uses a one-use, short-lived private activation page and an HttpOnly SameSite cookie. Owner writes require the expected Origin and CSRF protection. Agents use separate project-scoped profiles and sessions. Owner-only integration metadata and credentials are not agent capabilities. Product-private files use restrictive permissions; do not widen ACLs to bypass a tool sandbox.

Chat and agent-provided artifacts are untrusted data. Only formal owner records can answer their scoped questions. Exact decision, policy, generation, ownership, and candidate versions protect later work. A service record validates protocol references; it cannot certify that an agent actually ran Git or a test.

## Reporting a vulnerability

Use the repository's private vulnerability reporting facility if enabled. Otherwise contact the maintainer through their public GitHub profile without disclosing an exploit or secret in a public Issue. This project does not promise a response time or a security certification.

Do not upload tokens, activation files, cookies, complete databases, private paths, real chat exports, or unredacted traces. Share a synthetic reproduction, affected commit, expected/actual behavior, and sanitized logs. Rotate any accidentally exposed credential at its original provider; deleting a file from a later commit does not erase Git history.

Runtime data, local plans and guides, and rehearsal evidence are deliberately ignored. The public initial history starts from a sanitized snapshot. The publication checker is a guardrail, not a complete secret-scanning or legal-compliance certification.
