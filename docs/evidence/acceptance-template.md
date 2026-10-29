# Acceptance evidence format

This is a reporting procedure, not an execution report. Requirements are in the [design matrix](../../DESIGN.md#13-acceptance-requirements). Keep actual chats, owner files, tokens, cookies, private paths, trace exports and full databases outside Git.

Use opaque evidence: references and SHA-256 values for sanitized public summaries. The mapping to original files stays private. References and hashes alone do not establish truth; review actual calls, output, artifacts and observers.

```text
node scripts/acceptance-summary.mjs --self-test
node scripts/acceptance-summary.mjs --report PRIVATE_REPORT.json
```

Exit 0 means only self-reported-completeness-only. The checker requires all scenario evidence classes for accepted; m3Accepted excludes the separately authorized real external A29/A33 but still requires real hosts and clean-platform A36. It does not approve adoption, public release or deployment. Failed/skip/unverified are not passed.

Required real-host cases: A01/A04/A05/A07/A08/A11/A12/A13/A14/A20/A22/A23/A25/A35. A29/A33 require real-external, A36 real-platform, and the others synthetic. A01/A20 need actual calls from two different vendors, not two HTTP clients or two conversations of the same vendor.

## Deliberately incomplete example

```json
{
  "format": 1,
  "commit": "REPLACE_WITH_EXACT_CLEAN_40_CHARACTER_COMMIT",
  "toolchain": {"go": "ACTUAL", "node": "ACTUAL", "npm": "ACTUAL"},
  "lockHashes": {"go.mod": "SHA256", "go.sum": "SHA256", "web/package-lock.json": "SHA256"},
  "environment": {"os": "ACTUAL_BUILD", "arch": "amd64", "cpu": "ACTUAL", "browser": "ACTUAL_VERSION"},
  "bundle": {"target": "windows-amd64", "sha256": "ARCHIVE_SHA256", "sourceFingerprint": "MANIFEST_VALUE", "commit": "SAME_COMMIT", "dirty": false},
  "hosts": [],
  "witnesses": [],
  "cases": [{"scenarioId": "A20", "source": "real-host", "result": "unverified", "witnessIds": ["a20-calls"], "hostIds": [], "limitations": ["Two current hosts have not been exercised."]}]
}
```

Do not replace placeholders with arbitrary hashes to satisfy the checker. A host records id/vendor/host/version/toolTimeoutSeconds; this is the host tool limit, not the service wait interval. A witness records id/source/origin/reference/sha256/description/commit/bundleHash/hostIds. References must use evidence: aliases, not private paths. Bind each witness to the exact report artifact. Cases record scenarioId/source/result/witnessIds and nonempty limitations; real-host cases also bind hostIds. A20 includes integration baseHead/deliveryHeads/integratedHead/checkWitnessIds with exact hashes.

| source | origin | Scope |
| --- | --- | --- |
| synthetic | test-run, fixture or mock | Local checks only |
| real-host | host-tool-call | Actual tool calls in the named host |
| real-external | platform-delivery | Authorized provider result, not recipient reading |
| real-platform | os-observation | Actual clean OS/profile/device observation |

The normal CLI self-test uses incomplete, fabricated-source, stale-artifact, skipped, same-vendor and privacy counterexamples. These validate the report checker, not the product certification itself.
