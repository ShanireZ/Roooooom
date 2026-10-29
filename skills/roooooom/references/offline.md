# Offline, ambiguous results and restore

Attempt connection before project development. Network outage, SQLite busy or full permits independent, reversible work within existing human scope. Read cached state as stale and record local facts/checkpoints/artifacts privately. No cache means provisional work, not a global task/claim.

Do not gain a new role, claim a shared dirty checkout, assume an approval, or mark global completion while offline. Prefer an isolated branch/worktree permitted by the project. If a shared write scope may overlap, keep the artifact separate until coordination returns.

The native client uses a private durable SQLite outbox. Keep its original request ID, command metadata, cursor, occurred time and evidence; a network error is not success. Do not delete an unconfirmed item, replay it as a new request, or rewrite its epochs to match today's role.

~~~text
roooooom status --profile HANDLE --json
roooooom offline list --profile HANDLE --json
roooooom connect --profile HANDLE --project PROJECT_ID --session RECORDED_SESSION_ID --json
roooooom offline reconcile --profile HANDLE --json
~~~

Reuse a recorded session only for the same active conversation. A genuinely new work session omits that reuse flag and then explicitly resumes/hands off the responsibilities it needs.

Reconcile verifies identity, fully reads current tasks/claims/decisions and recovers an exact historical receipt when available. Explicit-recipient append-only chat may import as a late fact with original/received times. Role-targeted/control operations and stale work become candidates, never auto-claims/approvals/completion. Current-role review can reissue/retain/discard a candidate, but review itself does not execute its original operation; a new action uses current verified versions. Preserve originals and artifact provenance.

If serviceId or dataset generation changes, stop live mutations and request the owner's recovery path. Do not automatically enroll as another participant. The owner may issue a private recovery grant for the same participant:

~~~text
roooooom recover --profile HANDLE --credential-file PRIVATE_OWNER_GRANT_PATH --json
~~~

This adopts only that explicitly selected credential/session, retains drafts and rereads current state. It does not inherit responsibilities. Use an explicit session resume/handoff after reading the exact source session and current versions. Never read the owner's cookie/credential file to bypass recovery.

Revocation/401/403 is different from a network outage. Stop coordinated role work, preserve artifacts and ask the owner/integrator to resolve access/responsibility; do not bypass it with enrollment or another profile.

## HTTP-only private drafts

Use the same OutboxItem representation as the native client, in private product-controlled storage:

~~~json
{
  "localId": "local-random-id",
  "requestId": "original-random-request-id",
  "projectId": "known-project-id",
  "command": {
    "kind": "assignment.update",
    "targetId": "original-assignment-id",
    "meta": {
      "serviceId": "last-confirmed-service-id",
      "datasetGeneration": "last-confirmed-generation",
      "clientRequestId": "original-random-request-id",
      "sessionId": "recorded-session-id",
      "expectedVersion": 3,
      "claimEpoch": 1,
      "specVersion": 1,
      "acceptancePolicyVersion": 1,
      "decisionVersions": {}
    },
    "payload": {"state": "active"}
  },
  "cursor": "last-confirmed-cursor",
  "localOccurredAt": "2026-10-28T00:00:00Z",
  "authoritative": false
}
~~~

The sample records an unconfirmed intent. For first use leave unconfirmed service/generation values empty and keep a local provisional session ID; do not fabricate server identities. Keep credentials outside drafts. Store the exact actual command/metadata, not these sample values.

After authenticated full sync, POST offline.reconcile using a **new reconciliation request ID**, carrying localId, originalRequestId, originalCommand, baseCursor and localOccurredAt. Preserve the original command and proof. Delete the local copy only after a durable committed receipt/candidate acknowledgement; resolve task-scoped candidates through the current effective role. If the HTTP host cannot keep such storage private, retain independent artifacts and tell the human what remains unregistered.

The native client rejects known command-shape errors before staging new drafts. Legacy malformed drafts return a local `rejected` result and remain inspectable in `offline list`; later valid items still reconcile. This is neither a server candidate nor an executed operation. Correct the command under a new request ID after rereading current authority; do not change the original draft or discard an unknown network outcome. A profile with rejected/candidate items stays reconciling.

`response-too-large` (413) means a response exceeded the native client's 2 MiB wire bound, not that the service is offline. Normal metadata pages have a byte budget as well as a record limit. Narrow the query; if one record itself is too large, retain drafts and ask the owner to inspect a private export/reduce that record. Do not process unseen authoritative state as if full sync succeeded.
