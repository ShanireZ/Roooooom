# Roooooom design

This is the public product and protocol baseline. Field-level definitions live in [OpenAPI](api/openapi.yaml), same-version native machine help, and the [shared skill](skills/roooooom/SKILL.md). Verification status is separate from desired behavior; see [testing](docs/testing.md).

## 1. Purpose and boundaries

Roooooom is a single-user, single-machine coordination service. Existing coding agents connect using their own permitted command or HTTP tools. The service provides shared work records, messages, formal decisions, and integration responsibility. It does not host a model, intercept a vendor API, control agent processes, inject hooks, edit repository guides, execute Git, or independently run an agent's checks.

The user may create work in the board or in an existing agent conversation. The board never launches a host. Qualified agents discover work when they synchronize. A stopped or closed conversation cannot be awakened by Roooooom.

## 2. Architecture

- Go core and thin native CLI; SQLite with a single writer, durable receipts, and events.
- React/TypeScript browser application compiled into the core.
- A product-owned launcher and native Windows tray. Other candidate platforms retain a headless fallback until tested.
- Shared host-independent skill and an explicitly adopted project guide block.
- Optional outbound GitHub Issues projection and signed DingTalk notifications.

The listener is IPv4 loopback. Runtime databases, keys, profiles, activation pages, credentials, outboxes, backups, and exports live in a separate restrictive data root, never inside the installation. The service stores repository references; registering a path does not scan that repository.

## 3. Identity, authority, and mutation contracts

The owner activates using a private one-use capability valid for five minutes. It is submitted from a file form, never embedded in a URL. The browser receives an HttpOnly SameSite cookie; writes require expected Origin and CSRF protection. Expired activation gives an actionable recovery path. The product's Windows tray can open a fresh activation page. Owner-only connection metadata produces non-secret instructions without requiring manual IDs.

Enrollment is explicitly enabled for a named project. Each participant has scoped credentials; concurrent conversations use distinct profiles and sessions. A new connection does not silently resume old responsibility. Unknown presence is an observation, not evidence of a running host or permission to steal a claim. Revocation and restore invalidate the corresponding authority.

Every write uses service identity, dataset generation, clientRequestId and relevant object versions. The same actor/request identity and payload returns the committed receipt on retry; a different payload with the same request identity conflicts. Success is recorded only after commit. Events and unread references do not prove a model read or acted on them.

Agent-owned operations additionally use the current session, integrator epoch, claim/preparation epoch, specification, acceptance-policy, and decision versions where applicable. Owner-only actions are rejected before decoding an agent body. A shared name is display text; participant IDs remain the authority.

## 4. Human intent and executable work

A Request is durable human intent, not a Task with missing fields. The normal form has one required text field. Optional owner constraints, extra-collaborator maximum, designated integrator, and complete acceptance requirements remain attributable. Original text and explicit requirements cannot be rewritten by an agent.

Request states are waiting, preparing, converted, and cancelled. A qualified active project session atomically claims preparation. A preparer may release or offer it to another active session; the target explicitly accepts the current offer. A fresh session of the same participant may explicitly resume an ended source. Active or unknown sources require an explicit owner takeover; timeouts never grant ownership. Ending a session retains responsibility.

Request mutations carry its expectedVersion and preparationEpoch as claimEpoch. Only the current preparer can convert. Conversion verifies exact formal decision versions, owner requirements, project scope and the complete TaskCreate contract in one transaction, producing one unique linked Task. A lost conversion response cannot create another Task. After conversion the Task is the progress source; the Request preserves provenance.

The browser retains the complete first-submit payload for an unknown result. An explicit retry uses the same body and request identity even though optional form controls are disabled. Known rejection permits correction. Browser state is volatile, not a persistent mutation outbox, and writes never replay automatically.

## 5. Tasks, assignments, and capacity

Complete tasks include goal, scope, exclusions, actual authority references, risk, acceptance policy, and additional-collaborator maximum N. Placeholders derived from request text are not an executable contract. An initiating agent normally becomes integrator; an explicitly designated integrator is preserved. Human-created tasks are atomically claimed by the first qualified willing integrator unless designated.

N excludes the integrator, ranges from 0 through 64, and is a maximum rather than a required attendance count. One participant occupying several assignments does not consume multiple collaborator slots. N=0 allows one agent to implement and integrate. Work begins when its actual dependencies are satisfied, without waiting for every slot. Owner-set capacity cannot be overridden by agents.

Assignments define role, input/output contracts, write scope, repository, base reference and dependencies. One valid session claims each responsibility. A service claim is not a filesystem lock. Agents follow the repository's own isolation practices and preserve their branches on conflicts.

Detailed task phases are ready, active, integrating, review, done and cancelled. The daily board groups them into waiting, in progress and completed, retaining exact phases in details. Status follows authoritative records; no drag operation can manufacture completion.

## 6. Delivery, integration, and acceptance

Workers submit immutable revisions with exact repository/base/head, branch/workspace references, checks, unknowns and artifact references. Submitted does not mean accepted. The effective integrator reviews deliveries, performs Git integration using its own tools, runs checks against exact integrated heads, and records their outcomes and evidence.

Acceptance candidates bind the integration, specification, acceptance policy, decision versions and integrator epoch. Acceptance comes from the configured integrator, designated agent or human. Only the effective integrator completes a task after valid acceptance of the current candidate. Human acceptance alone does not close it.

New deliveries, contracts, heads, policy, specification, relevant decisions or ownership can invalidate candidates. Old evidence and completion snapshots remain history; they cannot complete a new version. Reopening retains prior records. Owner-specified acceptance requirements are protected from agent changes; permitted changes require reasons and invalidate stale evidence.

An integration handoff atomically accounts for every unfinished claim of the old integrator: transfer, release, or explicit continuation as a worker within N. Missing dispositions, stale epochs or exceeded capacity leave the whole transaction unchanged. The receiving session explicitly accepts a voluntary offer; forced takeover is owner-only.

## 7. Formal decisions

Questions contain facts, two or three options, recommendation and reason, impacts, blocked actions/assignments, work that can continue, and behavior while unanswered. Only affected work is blocked. A deadline does not approve it.

A question has exactly one taskId or intake requestId subject. Pre-task questions require the current preparer/version/epoch, reference no nonexistent assignments, and block request-convert. Cancelling the intake withdraws pending questions with history. Conversion adopts current decision versions without changing question IDs or original subjects. Later supersession reaches the linked Task and invalidates affected evidence.

Formal Decision.requestId identifies the question, not the intake. Only an authenticated owner resolves, withdraws or supersedes. Chat, agent relays, titles, Issues and imported logs cannot become human authority. Required authority-document follow-up remains blocked until its evidence is recorded.

The UI requires an explicit option selection and confirmation; the visible label forms the formal answer, with optional supplement. Custom answers and withdrawal require text. A prepared specProposal or externalProposal does not change scope or grant permission. Formal changes still verify versions and owner requirements. External authorization records exact action, target, scope, confirmed wording, expiry and bounded uses; the service does not execute the external action.

## 8. Messages and timely processing

Messages preserve the actual actor, occurrence/receipt times, body, purpose, priority and recipient snapshot. Role recipients bind at commit; later role changes do not redirect old messages. Normal chat defaults to discussion/normal/no action required. Requesting handling is explicit and does not substitute for a formal decision.

Receipt meanings remain distinct: stored, fetched, seen, accepted/declined, and resolved. Decoding a returned body may establish fetched. Agents mark seen after reading, accept/decline after deciding how to handle it, and resolve with evidence. Transport delivery never means model processing.

Agents synchronize before work, after bounded segments or long tools, and before shared-interface changes, delivery, integration, acceptance and completion. Business responses include unread references. Finite waits are at most 60 seconds with an initial ten-minute idle budget; explicit renewal requires an active reason. Checkpoints communicate current work, blockers and next steps. There is no background helper that wakes a host.

## 9. Offline work and reconciliation

Outage permits isolated local development. Existing responsibility snapshots may guide work but cannot establish new global claims or approvals. Without a cache, work is provisional. Native clients retain immutable original mutation identity, versions, local time, artifacts and drafts in a private durable outbox.

Reconnect first verifies identity/generation and synchronizes. Exact safe retries can recover their original receipt; stale or authority-changing actions become conflicts or review candidates, not silently rewritten operations. Old session/epoch/decision values cannot be replaced merely to make replay succeed. Repeated interruption retains individual reconciliation outcomes.

The browser keeps an in-memory read-only snapshot and last synchronization time, automatically attempts read recovery and revalidates before enabling writes. Unknown human writes need explicit retry. Revocation, generation reset and invalid cursors require the appropriate explicit recovery.

## 10. Optional external records and notifications

Roooooom remains task authority. GitHub Issues are a one-task/one-Issue projection of allowed summaries, milestones and completion; remote edits do not overwrite local task state. No Issue is created for an unconverted intake.

GitHub setup accepts an official repository URL or owner/repo and a repository-limited fine-grained PAT. Owner-only binding.inspect performs a bounded, timed metadata GET against api.github.com without redirects. It returns canonical repository/id/private/hasIssues and issuesWriteVerified=false. Metadata access cannot prove Issues write permission. The owner confirms the destination and outbound content before enabling. The product never reads another tool's GitHub login.

Jobs retain their original binding, target, marker and version. Unknown writes are reconciled instead of blindly recreated; rotation does not erase uncertainty or move old jobs. Known authorization/rate failures and uncertain outcomes are distinguished. Restore disables bindings until explicitly confirmed.

DingTalk setup parses only the official HTTPS robot/send URL with one access_token and requires a signing secret. Arbitrary endpoints, userinfo, extra query fields, ports, fragments and redirects are rejected. Credentials stay product-private; responses contain references only. Saving or enabling performs no test send.

Notifications cover pending human decisions, ready human acceptance and owner-actionable blockers. Stale/solved items are suppressed before delivery. Platform acceptance is not recipient reading. Shared bot rate limits and bounded retry apply; a timeout may duplicate a notification identity. Mobile recipients must return to the local decision page on the host machine; a group reply is never a formal decision.

## 11. Browser workflow

Each registered project is a room. The detailed owner-interface design, including the visual system, is in [docs/ui-design.md](docs/ui-design.md); this section states the shipped behavior.

- **Shell.** Room tabs with per-room owner counts, a global Needs you list, the connection state and system settings. `/` opens the last visited room in this browser, then the first room with owner items, then the first room; with no rooms it opens room creation. `/projects`, `/decisions`, `/tasks/:id`, `/requests/:id` and `/setup` redirect into rooms; an unknown record shows not-found inside the shell.
- **Floor.** Requests waiting for preparation are notes at the door. Tasks are tables showing phase, progress, owner flags and the seats responsible for them; flagged tables come first. A seat has a stable per-agent color and initials, and a seat not synchronized for 15 minutes is dashed with its last-synced time. Done and cancelled work goes to the file cabinet, which opens a searchable archive. Members without a seat wait in the lobby. An activity feed of chat and formal events sits beside the floor from 1200 px and behind a Floor | Activity switch below; its composer asks agents for work or posts chat. The floor fills the window height, and from 1200 px an open drawer takes the activity column's place while the floor reflows rather than being covered.
- **Drawers.** Task, request, formal question, acceptance, archive and room settings drawers are addressed by path, so Back closes them. The task drawer has Overview, Work, Evidence, Discussion, History and Manage tabs. Identifiers, versions, epochs and exact heads stay in Technical details folds.
- **Needs you** lists formal questions, valid ready human candidates and owner-actionable blockers across rooms; after resolving one it offers the next item.
- **First use** is one screen: a repository reference, an editable room name, the consent sentence and Open the door. Non-secret join instructions in the interface language follow, with a live line that changes when the first agent joins; Roooooom never starts or wakes agents. A registered room whose door is closed shows a banner to open it, so setup is never duplicated. Integrations never appear in first use.
- **Room settings** hold the door (repository references and join instructions), members (sessions, private recovery files, and revocation after an inline confirmation) and integrations (guided and advanced GitHub and DingTalk setup, credential rotation, disable, and job retry or reconcile). **System settings** at `/settings` hold the owner language, theme, backup and export, storage, restore guidance and build information.
- **Language.** The UI is available in English, Simplified Chinese, Traditional Chinese and Japanese. The owner language, stored by the service through an owner-only command, takes precedence over browser detection and also selects the activation page, the expired page, tray labels and new DingTalk notifications. Agent-authored text is shown verbatim and never translated.

Same-name choices include a short identifier. Session observations describe last synchronization, never guessed host activity. Management forms bind the version shown when opened. A concurrent change disables the stale form until it is reopened. User-entered requirements survive unknown-result retry. Long and multilingual content is rendered as text; chat does not execute Markdown or HTML.

## 12. Storage, recovery, and packaging

Protocol version remains 1; durable schema is 2. Forward schema migration preserves service identity, generation and existing history and checks integrity/foreign keys. Older binaries reject schema 2. A binary downgrade alone is not a data-preserving rollback.

Consistent SQLite snapshots include committed WAL state. Readable exports exclude credentials. Restore requires the identified snapshot/service, changes dataset generation, invalidates old credentials/cursors/candidates and requires explicit recovery and binding confirmation. The database has a default one-GiB cap and a 75% warning; WAL, backups, exports and free disk space are separate concerns. Storage busy/full and commit ambiguity remain explicit.

The package contains the core, launcher, embedded UI, same-version contract, shared skill, dependency notices and GPL license. Build/source/UI hashes and checksums prove byte relationships, not trusted OS signing. Closing a browser leaves the service alive; the product can be stopped explicitly. Data roots cannot be inside installation paths. Platforms stay unsupported until their required native and clean-machine evidence is complete.

## 13. Acceptance requirements

Automated protocol and browser tests do not replace actual vendor-host, external-provider or clean-platform witnesses. Scenario IDs are stable for the report checker; detailed manual procedures are under tests/scenarios. The following is a requirement matrix, not a completion report.

| ID | Scenario | Required observation |
| --- | --- | --- |
| A01 | New participant from existing hosts | Two different vendors; no host or model-credential modification |
| A02 | Concurrent assignment claim | One owner or explicit distinct assignments |
| A03 | Reused request identity and lost response | Original receipt, no duplicate resource |
| A04 | Worker message while integrator waits | Committed event returned without fabricated reading |
| A05 | Idle, long command or ended conversation | Message retained; no wake-up promise |
| A06 | Commit between empty read and wait | Recheck captures it, replay deduplicates |
| A07 | Multiple worker deliveries | Exact-head integration before completion |
| A08 | Integrator handoff and old epoch | Unique new owner, old writes rejected, artifacts retained |
| A09 | Changed heads/policy/decision after integration | Stale evidence cannot complete new work |
| A10 | Agent attempts human authority | Rejected; quoted approval grants nothing |
| A11 | Owner decision and supersession | Scoped unlock, correct version propagation, history retained |
| A12 | Existing assigned work during outage | Explicit offline mode and durable local drafts |
| A13 | New work without cache | Provisional only, no global claim or approval |
| A14 | Ownership/decision changed while offline | Reconcile conflicts rather than overwriting current state |
| A15 | Outage during reconciliation | No duplicate replay or lost outbox |
| A16 | Restore, cursor reset and SSE failure | Full new-generation sync and retained unread history |
| A17 | Disk full/busy and commit crash | No success before commit; ambiguity recoverable |
| A18 | Guide adoption, repetition and manual edits | No duplicate block, no overwriting neighbors |
| A19 | Board authority and malicious local requests | Normal creation works; bypass/cross-scope writes fail; text stays safe |
| A20 | Two vendors complete one task | Traceable split, chat, deliveries, integration, checks and decisions |
| A21 | Agent versus human task initiation | Correct initial integrator without coordinator-role deadlock |
| A22 | Ended, concurrent or unknown source sessions | Explicit resume; epoch fencing; no automatic takeover |
| A23 | Competing or designated integrators | One eligible claim, preserved designation, no host launch |
| A24 | Collaborator-capacity races and reductions | N never exceeded; sessions do not double-count a participant |
| A25 | Partly filled N, N=0 and dependencies | Independent work proceeds; only real dependencies block |
| A26 | Relayed versus formal human answer | Only owner page resolves the exact question/version |
| A27 | Issue/task disagreement | Local task remains authoritative |
| A28 | Designated/human acceptor and changed heads | Current authorized acceptance required |
| A29 | GitHub writes, timeout, retry and human edits | One reconcilable Issue; no blind duplicate or remote-to-task overwrite |
| A30 | Expired PAT, limits, target switch and restore | Queue/binding recovery without scope expansion |
| A31 | DingTalk errors, signatures and limits | No false success, bounded retry and redaction |
| A32 | Duplicate/stale notifications and backlog | One version/channel job, suppress irrelevant notifications |
| A33 | Real mobile notification and local-only response | No false remote-access or group-approval claim |
| A34 | Protected owner acceptance requirements | Owner-only changes; valid agent changes invalidate old evidence |
| A35 | Incomplete handoff dispositions or capacity overflow | Atomic rejection or complete valid transfer |
| A36 | Clean-machine install/reopen/upgrade | No development runtime dependency, isolated data, explicit lifecycle and recovery |

Daily-workflow requirements additionally cover: guided resumable enrollment (UX01), one-field durable intake (UX02), fenced preparation/complete conversion (UX03), named contextual UI (UX04), simple formal answers with full authority (UX05), ordinary text chat (UX06), optional safe integrations (UX07), explicit recovery (UX08), real-browser narrow/keyboard/empty/duplicate-name cases (UX09), and matching contracts/artifacts without inflated support claims (UX10).
