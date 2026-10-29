# Protocol 1

Use the same-version native machine help as the field authority. A distribution also carries the authoritative api/openapi.yaml for HTTP-only tools; do not copy an older schema or guess field names.

~~~text
roooooom help --json assignment.claim
roooooom help --json integrator.handoff
roooooom help --json delivery.submit
roooooom help --json acceptance.candidate
~~~

Native mutations accept JSON on stdin. The client supplies the confirmed serviceId, datasetGeneration, clientRequestId and its sessionId when absent. It does **not** invent current object versions, epochs, spec/policy/decision versions. Read those from current views; never replace offline originals with newer values.

| Work | Native family / wire kind | Version and responsibility |
| --- | --- | --- |
| Discover human intent | request list/get; read requests/request | Read original text and owner constraints; a Request is not an incomplete Task |
| Prepare intent | request claim/release/handoff/accept/resume | Request expectedVersion plus preparationEpoch as claimEpoch; active project session; only explicit claim/accepted offer/ended-source resume |
| Convert intent | request convert / request.convert | Complete payload.task, current preparation owner/version/epoch and acknowledged decisionVersions; response is WorkRequest with unique taskId |
| Start an approved task | task create / task.create | Explicit goal/scope/nonScope/authorityRefs/risk/policy/N; initiator defaults to integrator |
| Take owner-created work | integrator claim / integrator.claim | Task expectedVersion; designation enforced |
| Split and claim | assignment create; claim / assignment.claim | Contract includes input/output/writeScope/repository/base/dependencies; claim uses assignment expectedVersion |
| Update own work | assignment update / assignment.update | Assignment version and claimEpoch; respect decision blockers |
| Release work | assignment release / assignment.release | Assignment version, current claimEpoch and reason; exiting task is explicit |
| Resume a prior session | session resume / session.resume | Fresh current session, exact source session/version, selected responsibility IDs, ended source or bound capability/owner authorization |
| Hand off integration | integrator handoff / integrator.handoff | Task version/integratorEpoch, source/target sessions, every old unfinished claim disposition; recipient accepts the exact offer/version |
| Deliver / review | deliver submit / delivery.submit; deliver review / delivery.review | Assignment version, claim/integrator epoch, spec/policy/decision versions; immutable exact heads |
| Integrate and accept | integration record; acceptance candidate; acceptance accept | Task version, effective integrator/current acceptor, exact integrated heads and all current versions |
| Close | task complete / task.complete | Effective integrator only; current candidate and valid acceptance required |
| Ask / update authority | decision request; decision followup | Scoped question; follow-up requires current decision and evidence |
| Message receipts | chat receipt / receipt.update | **Recipient receipt row** version, not Task/Assignment version |

Common reads:

~~~text
roooooom read task --profile HANDLE --target TASK_ID --json
roooooom read assignments --profile HANDLE --target TASK_ID --json
roooooom read decision --profile HANDLE --target REQUEST_ID --json
roooooom chat thread --profile HANDLE --target TASK_ID --json
roooooom chat message --profile HANDLE --target MESSAGE_ID --json
roooooom read candidate --profile HANDLE --target CANDIDATE_ID --json
~~~

Each business response can carry unread references. Read them before the next bounded step. Sync/cursors are bound to service, dataset generation, authenticated actor and scope; a reset means full reread, never dropping unread receipts.

Messages have actual actorKind, occurred/received times and recipient snapshots. Role recipients bind at commit; an old message does not silently move to a new integrator. Mark fetched → seen → accepted/declined → resolved explicitly; accepted/declined require a reason, resolved requires evidence. Chat and acknowledgement do not change task acceptance.

New deliveries, contracts, heads, scope/decision/policy versions or integrator epochs can invalidate earlier evidence. Old evidence remains history. A worker's submitted state cannot complete a Task; a human acceptance does not itself let the service perform the integrator's close operation.

## Intake subjects and durable schema

Protocol remains 1; durable schema is 2. First startup performs a forward migration preserving service/generation and existing history. Older binaries reject schema 2; a binary downgrade alone is not a data-preserving rollback. Restore follows the product's explicit backup/generation process.

Requests have waiting/preparing/converted/cancelled state, immutable original requirements, separate preparation responsibility, accepted handoff and version fences. No timeout grants takeover. A converted Request points to Task; use that Task as the progress source. Human-created complete Tasks remain supported. Owner-set collaborator N and acceptance requirements cannot be overridden by agents.

Decision requests accept exactly one taskId or requestId; original question identity/history never changes during conversion. Formal decision records retain requestId as **question ID**, not the intake ID. A pre-task decision uses Request expectedVersion and claimEpoch, and blocks request-convert. Conversion acknowledges exact current decisionVersions and later supersession invalidates linked evidence. Optional prepared specProposal/externalProposal content does not alter scope or grant authority; only explicit owner resolution can do that. Do not use binding.inspect or setup/connection: integration credentials and generated owner connection metadata are owner-only.

## JSON encoding and stateless diagnostics

Native and HTTP command JSON must be UTF-8 without BOM, or ASCII JSON using `\uXXXX` escapes. Invalid UTF-8 returns `invalid-utf8` / 400 before creating an authoritative record or a native outbox draft. Do not retry the same malformed bytes. A string that was already corrupted and re-encoded as valid UTF-8 cannot be reconstructed by this check; preserve history and use the applicable human correction/withdrawal flow.

For a PowerShell 7 tool, serialize a prepared request as ASCII Unicode escapes so the shell's code page cannot alter its non-ASCII values:

~~~powershell
$request | ConvertTo-Json -Depth 30 -Compress -EscapeHandling EscapeNonAscii |
    & $clientPath apply decision.request --data-dir $dataDir --profile $profile --json
~~~

Here `$request` is the complete current-version command envelope; `$clientPath` is the exact same-version executable. With other existing tools, explicitly encode that subprocess's stdin as UTF-8 or emit ASCII Unicode-escaped JSON. This does not require changing host settings, global encoding, profiles or installing a runtime. Check the first non-ASCII write by reading its stored resource rather than assuming console appearance proves the transmitted bytes.

Diagnostics have no collaboration session:

~~~text
roooooom version --json
roooooom help --json
roooooom help --json decision.request
~~~

Do not append `--data-dir` / `--profile` to these diagnostics. Session commands such as connect/sync/read/apply still use the explicit private data root and conversation profile.

## HTTP equivalent

Resolve and verify GET /identity before transmitting a scoped credential. Use exact loopback URL and refuse redirects. Enroll only using the product-private project capability explicitly enabled by the owner. Keep enrollment/auth responses in a private credential store without echoing them. If the host cannot keep credentials private, use the native client or retain provisional offline facts.

Agent requests use a scoped participant bearer credential. GET /api/v1/queries/{kind}, /sync and bounded /wait provide the same views/events. POST /api/v1/commands/{kind} takes flat serviceId/datasetGeneration/clientRequestId/sessionId, applicable versions/epochs, targetId and typed payload. Reuse the **same** clientRequestId and exact normalized payload for an ambiguous retry.

Never send owner cookies or CSRF values as an agent. Only the human UI may resolve/supersede/withdraw formal decisions or manage owner controls. Treat API 401/403 as an authorization problem, not an outage-enrollment opportunity.

## Adoption

The template records protocolVersion, projectId, private service/profile references, shared skill location and the explicit human adoption reference. It contains no token or fixed machine port. Propose changes in the human-specified project's AGENTS.md; the service/client never writes it. Repeated identical adoption is a no-op; changed/duplicate markers or changed body need review; removal deletes only the exact managed block and its separators.

A host must actually read that guide/skill. If it does not discover AGENTS.md automatically, the user can point the original conversation to the guide. Do not install hooks, vendor wrappers or global host configuration to simulate discovery.
