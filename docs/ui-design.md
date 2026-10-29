# Owner interface redesign

Status: design approved by the owner on 2026-10-29. Implemented in 8a5fbe8, with the §10 refined blueprint revision implemented in 63d85df; [DESIGN.md](../DESIGN.md) §11 is authoritative for shipped behavior. Protocol and authority rules in DESIGN.md are unchanged. Additive server reads, one owner preference command, and a possible forward schema migration are allowed only as described in §16.

## 1. Intent

The owner's job in Roooooom is small and recurring: leave a short request, see who is responsible for what and where it is stuck, answer formal questions, accept results when asked, and occasionally manage people, integrations and recovery. Agents do everything else in their own hosts.

The previous browser workflow already limited daily navigation to Projects and Needs my attention, but the owner reported four problems with it:

1. It is hard to see what agents are doing: who holds which responsibility, where work is stuck, and what happens next.
2. Decisions and acceptance are too technical: candidate IDs, SHAs, epochs and versions dominate the screen.
3. Information is scattered across stacked panels and nested disclosures.
4. Enrollment and setup take too many steps.

The redesign gives each project a **room** drawn as a floor plan, uses one predictable **drawer** for all details and owner actions, puts plain language first with technical detail one layer down, and gives the product a distinctive **blueprint** identity. All owner-visible text is available in English, Simplified Chinese, Traditional Chinese (Taiwan) and Japanese.

## 2. Goals, non-goals and anti-goals

Goals:

- One glance at a room shows every open piece of work, who is responsible for it, how far it has progressed, and whether it waits for the owner.
- Every owner action (request, formal answer, acceptance, management) is reachable from the room in at most two interactions, and every detail (specification, evidence, history, technical identifiers) is in the same drawer in a predictable place.
- Formal decisions and acceptance read as plain language. Exact technical bindings stay visible on demand.
- First use goes from an activated owner to join instructions on one screen.
- The interface looks like itself, not like a generic admin template.
- All owner-visible text is available in four languages.

Non-goals:

- Changing authority, version fencing, receipts, retry, offline or reconciliation semantics (DESIGN.md §3–§9).
- Starting, waking or monitoring hosts; showing live presence; remote or mobile access to the local service.
- Rendering Markdown or HTML in chat or agent-authored text.
- Translating stored content written by the owner or agents. Only interface chrome and system-generated text are localized.
- Localizing agent-facing CLI output, machine help, JSON, or the shared skill.

Anti-goals, chosen by the owner. The design fails even if it works when:

- it looks like a generic SaaS back office;
- details become so hidden that evidence, history or versions are hard to find;
- the boundary blurs between the owner's formal decisions and chat, relays, or agent proposals.

## 3. Constraints

- The single-user loopback service embeds a React/TypeScript UI in the Go binary. The UI works offline: no network fonts, CDNs or remote assets.
- The write-safety behavior in `web/src/api/` stays as it is: a reused clientRequestId on explicit retry, the first-submit payload kept for unknown results, version-bound forms disabled when stale, read-only operation until synchronized, and no automatic write replay.
- Presence is an observation. The UI shows the last synchronization time, never "online", "working" or "here".
- Chat and agent-authored text render as plain text.
- No new runtime dependencies. React 19 and React DOM remain the only runtime packages.
- Server changes are limited to §16. Any new query or command also appears in the agent-facing machine help generated from the OpenAPI contract, so it must be named and described as carefully as existing entries.
- Pinned toolchain and gates follow [CONTRIBUTING.md](../CONTRIBUTING.md). UI changes need real-browser checks.
- Prose documentation stays in English. Product UI strings are not documentation translations.
- Old browser paths keep working through redirects. People may have bookmarks or history entries, and notifications tell recipients to return to the local decision page.

## 4. Approaches considered

| Approach | Summary | Trade-off |
| --- | --- | --- |
| A. Rebuild the UI layer in place, no new dependencies (**chosen**) | Keep `web/src/api/`. Rewrite `features/` and styles as room, drawer and list structures on a design-token stylesheet. Use the native `<dialog>` and popover APIs for the drawer and the Needs-you list. Implement i18n as typed in-house catalogs plus the built-in `Intl` APIs. Draw the floor plan with CSS Grid over semantic lists. | Accessibility details are hand-built and must be tested. It fits the project's minimal-dependency practice and leaves the write-safety code untouched. |
| B. Adopt a component library and an i18n library | For example, React Aria with Lingui or i18next. | Accessible primitives and translation tooling come ready-made, but the project gains 3–5 dependencies with notices, size and pin maintenance, and every component still needs a full blueprint restyle. |
| C. Incremental reskin | Restyle and extract strings first, then convert page by page. | Each step stays usable, but old and new structure coexist for a long time and the page-to-drawer change gets fragmented. Total effort is highest. |

The owner chose A because the information architecture changes anyway, so a reskin saves little. A also keeps the dependency surface and the safety layer unchanged.

## 5. Owner decisions

Recorded 2026-10-29 in the design conversation. The owner chose each item from proposed options:

| # | Decision |
| --- | --- |
| O1 | Scope: new visual identity **and** a rethought flow and information architecture. All four reported problems are in scope. |
| O2 | The home of daily work is a project **room**. |
| O3 | Rooms use the **floor-plan** layout. Each open task is a table, and agents sit at the tables they are responsible for. |
| O4 | Completed tasks go into a corner **file cabinet** (count plus the three most recent; opens a searchable archive). |
| O5 | Unconverted requests are **notes at the door**. A note becomes a table when converted. |
| O6 | Rooms switch through **tabs at the top**. A global **Needs you** control sits at the top right. |
| O7 | A seat whose session has not synchronized for a while turns into a **dashed outline** with the last synchronization time. Responsibility stays at the table. |
| O8 | Visual style is **blueprint**, with a **distinct color per agent**. The palette in §10 is accepted. |
| O9 | Needs you opens as a **dropdown list** grouped by room. An item opens the room and its drawer. |
| O10 | Decisions and acceptance show **plain language first**. Technical detail sits one layer down in the same drawer. |
| O11 | First use is **one screen** that creates the room: reference, name, open the door, copy instructions. GitHub and DingTalk move to room settings. |
| O12 | UI language **follows the browser**, with a manual override. |
| O13 | Implementation approach A (§4). |
| O14 | The four languages cover **all owner-visible text**: Web UI, activation and expired pages, tray menu, and DingTalk notifications. Agent-facing CLI/JSON is excluded. |
| O15 | Screens not mocked up (activity and chat, other drawer tabs, settings, narrow layout) are specified here and validated in a real browser during implementation. |
| O16 | This document is public, in English, committed locally. Mockups stay in ignored local evidence. |
| O17 | The server may gain the additive reads and the owner language command in §16. A forward schema migration (2 → 3) is allowed **when necessary**, under the rules and recovery plan in §16.3. |
| O18 | Palette correction: agent plum is replaced by fuchsia (`#a21caf` / `#f0abfc`), keeping agent colors clear of the danger hue. |

## 6. Information architecture

### 6.1 Shell

The top bar holds, left to right:

- the wordmark;
- one tab per room, showing the room name and the count of owner items in that room;
- `+` for a new room;
- connection state (online, re-synchronizing, or read-only with the last synchronization time);
- **Needs you** with the global count;
- a gear for system settings.

There is no sidebar.

### 6.2 Routes

| Path | Screen |
| --- | --- |
| `/` | Redirects to the last visited room (per-browser `localStorage`). Without one, it goes to the first room that has owner items, then the first room by creation order. With no rooms, it goes to `/rooms/new`. |
| `/rooms/new` | First use or new room (§9). |
| `/rooms/:projectId` | Room floor (§7). |
| `/rooms/:projectId/tasks/:taskId[/:tab]` | Room with the task drawer open; tab is `overview`, `work`, `evidence`, `discussion`, `history` or `manage`. |
| `/rooms/:projectId/requests/:requestId` | Room with the request drawer open. |
| `/rooms/:projectId/decisions/:decisionRequestId` | Room with the formal question drawer open. A question with a request subject resolves to that request's project. |
| `/rooms/:projectId/acceptance/:taskId` | Room with the acceptance drawer open for the task's current candidate. |
| `/rooms/:projectId/archive` | Room with the archive drawer open. |
| `/rooms/:projectId/settings[/:section]` | Room with the room-settings drawer open; section is `door`, `members` or `integrations`. |
| `/settings` | System settings (§8.7). |

Legacy redirects: `/projects` goes to `/`. `/decisions` goes to `/` with the Needs-you list open. `/tasks/:id` and `/requests/:id` look up the project and go to the room path. `/setup` and `/setup/new` go to `/rooms/new`. `/setup/:id` goes to `/rooms/:id/settings/door`. An unknown or missing object shows a not-found state inside the shell.

Drawer state lives in the path, so Back closes the drawer or steps back through drawer navigation, and links are shareable on the same machine.

### 6.3 Needs you

The list shows, grouped by room and ordered oldest first within each room:

- pending formal questions, with a task or request subject;
- tasks whose acceptance policy names the human owner and whose current, non-stale candidate the owner has not yet accepted;
- owner-actionable blockers. Today these are the attention codes `integrator-revoked`, `integrator-session-ended`, and `pending-reconciliation:*` on open tasks, plus ended or revoked preparers on open requests.

Each item shows its stamp (§10.4), a one-line plain summary, what it blocks, and its age. Selecting an item routes to the matching drawer. After the owner resolves an item, the drawer offers **Next item** and moves to the next one in list order; the room changes if needed. An empty list says nothing needs the owner and notes that work continues without them.

## 7. Room

### 7.1 Regions

```text
+-----------------------------------------------------------+------------------+
| DOOR      | FLOOR: open task tables              | CABINET | ACTIVITY         |
| request   |  [table] [table] [table]             |  done   | feed + filters   |
| notes     |  [table] [+ ask agents to do ...]    | ------- |                  |
|           |                                      | LOBBY   | composer         |
+-----------------------------------------------------------+------------------+
```

- **Door**: requests in `waiting` (no preparer) and `preparing` (the preparer's seat shown on the note). Converted requests leave the door: the note becomes a table, with a short transition when motion is allowed. Cancelled requests appear only in the archive.
- **Floor**: tasks in `ready`, `active`, `integrating` and `review`. A permanent dashed **+ Ask agents to do something** table opens the request composer.
- **File cabinet**: count of `done` tasks plus the three most recent goals. It opens the archive drawer, which lists done and cancelled tasks and cancelled requests with text search and a status filter.
- **Lobby**: below the cabinet in the same column; room participants who hold no open responsibility, with their last synchronization time. Revoked participants never appear in the lobby or the members list, because revocation removes their project membership. A seat that still references a revoked participant shows "Revoked agent" with the short identifier and carries the ⚠ blocker flag.
- **Activity**: the room feed (§7.5).

### 7.2 Tables

A table is a sheet about 1.6:1 (at least 168 px tall) and shows:

- the goal (wrapped to three lines, then truncated with the full text available);
- the phase in plain words;
- a segmented progress bar with one segment per assignment, colored by state: accepted, in progress, waiting for the owner, blocked by dependency, or open. A task without assignments shows a single segment for the integration state;
- seats, inside the table along its bottom edge, so tables in one row line up.

**Seats** appear once per participant per table, never once per assignment. This matches the collaborator rule in DESIGN.md §5 that one participant never counts twice. Each seat shows the agent color and initials, a role label (`integrating` outranks a worker role; a worker seat shows its assignment role or `delivered`), and the last synchronization time as relative text. Same-name participants add the short identifier used today.

**Flags** sit on the table edge. The table border takes the warning color only for owner items.

| Condition | Flag |
| --- | --- |
| Pending formal question whose subject is this task | ⚖ Waiting for you |
| Human acceptance ready (as in §6.3) | ✓ Ready for your acceptance |
| Owner-actionable blocker (as in §6.3) | ⚠ Needs your action |
| No integrator yet | Empty chair: "Waiting for an agent to take this" |
| `canStart` false without an owner item | Muted "Waiting on dependencies" line |

**Stale seats** follow O7. When a responsible session's `lastSeenAt` is more than 15 minutes old, the seat becomes dashed and its text reads "Last synced 1 h ago". An ended session reads "Session ended". Stale styling never removes responsibility and never says "offline". The threshold is a single UI constant.

**Ordering**: owner-flagged tables first, then other open tables by most recent activity, then `ready` tables. Order is computed when data loads and does not shift while the pointer hovers or a table has focus.

**Density**: comfortable tables up to eight open tasks, compact tables (goal, bar and seats only) beyond that. The floor scrolls vertically and never horizontally.

**Member focus**: selecting a seat or a lobby member highlights that member's tables and dims the rest until the selection is cleared. This replaces the old participant filter.

### 7.3 Door notes

A note shows the request text (three lines), its state in plain words, and, when preparing, the preparer's seat with the last synchronization time. Selecting a note opens the request drawer. Pending questions with a request subject flag the note with ⚖.

### 7.4 Empty and first states

- A room without agents: the lobby says no agent has joined yet and offers **Copy join instructions**, which opens room settings at the door section.
- A room without tasks or requests: the floor shows only the add table, with one line explaining that agents pick up requests when they next synchronize, and that Roooooom does not start them.

### 7.5 Activity feed and composer

The feed is chronological, newest at the bottom, and loads older entries on scroll. Entry kinds:

- owner requests;
- preparation claimed and request converted;
- assignments created and claimed;
- deliveries submitted;
- integrations recorded;
- formal questions raised, resolved, withdrawn or superseded;
- acceptance and completion;
- chat messages;
- offline-imported records, stamped with both occurrence and receipt times.

Filter chips: All, Chat, Progress, Decisions.

Display rules:

- Chat entries carry a small "chat" label and never look like formal entries.
- Formal-question entries use the formal stamp and link to the decision drawer.
- An agent message that merely relays or quotes an approval stays a chat entry.

The composer at the bottom has two modes:

- **Ask agents to do something**: the one-field request form, with an optional **More requirements** fold (owner constraints, collaborator maximum N, designated integrator, acceptance conditions and acceptor). It keeps today's unknown-result retry behavior and offers **I already have a complete specification**, which opens the full task-create form in a drawer.
- **Say something**: a project-thread chat message. The **Ask the responsible agent to handle this** toggle and the purpose and recipient fold behave as today. Message receipts show in a per-message detail view.

Whether the feed is assembled from existing queries or from a new read-only `activity` query is the builder's call within §16.

## 8. Drawer

### 8.1 Behavior

There is one drawer at a time. On wide layouts it opens from the right over the activity column (420–560 px). On narrow layouts it is a full-screen modal. It uses `<dialog>`:

- non-modal on wide layouts, so the floor stays usable;
- modal on narrow layouts;
- focus moves to the drawer heading;
- Escape and the close control return focus to the element that opened it.

Every drawer ends with a **Technical details** fold that holds identifiers, versions, epochs, heads and receipt states. Version-bound forms keep the DESIGN.md §11 rule: when the shown version changes, the form is disabled with a **Reopen** action.

### 8.2 Task drawer

The header shows the goal, the phase in plain words, the responsible seat, and one **Next step** sentence derived from state, such as "Waiting for your decision on table-of-contents position" or "Codex is integrating 2 deliveries".

Tabs:

- **Overview**: the current result summary and its untested scope, when a non-stale candidate exists; what blocks progress; collaborators as "2 of up to 3"; the original request text and owner constraints, when converted from a request; and GitHub record links.
- **Work**: assignments with role, seat, state, dependencies and whether each can start. A fold holds write scope, repository, base reference, contract version and claim epoch. Actions: **Add assignment** and **Cancel assignment** (with reason), with today's fields and fences.
- **Evidence**: deliveries, integration attempts, acceptance candidates and completion snapshots as plain summaries ("Claude Code delivered revision 2: TOC component · 3 checks reported"). Each entry opens a fold with exact heads, checks, evidence references, unknowns and artifacts.
- **Discussion**: the task thread with the same composer rules as §7.5.
- **History**: formal decisions for this task with their versions and outcomes, plus the task's activity entries.
- **Manage** (owner-only): change the collaborator maximum, cancel or reopen, edit acceptance rules, and force an integrator handoff with per-claim dispositions. Each action requires a reason. Destructive actions use the danger style and an explicit confirmation, and every action keeps today's version fences. Acceptance-rule editing moves here from the decisions page and is also linked from the acceptance drawer.

### 8.3 Request drawer

Shows the request text, its state, the preparer seat with last synchronization, owner constraints, linked questions, and the linked task once converted. Manage covers reassigning the preparer to an active session and cancelling, both with a reason and today's fences. The raw record goes in Technical details.

### 8.4 Formal question drawer

In order:

1. The formal stamp, "only you can answer", and the subject (task or request).
2. The question and its plain context (`reason`).
3. The agent recommendation, attributed to its author and visibly the agent's words.
4. The options as large radio cards, each with its label and impact; the recommended option is marked.
5. **Other decision**, with the outcome select; free text is required.
6. Folds: **Supplementary note** (optional); **Basis, impact, what is blocked, what can continue, what happens if unanswered, deadline**; Technical details.
7. A visible, labeled section titled "This question can also approve: …". It follows the server's acceptance rules, not the presence of a proposal:
   - The spec-amendment part appears when the question blocks `spec-change` and has a task subject. Its fields are prefilled from `specProposal` when present, otherwise from the task.
   - The external-authorization part appears when the question blocks `external-action`. Its fields are prefilled from `externalProposal` when present, otherwise blank.
   - The section is never inside a fold. Each part has an explicit opt-in toggle that is off by default and reveals its fields. When a proposal exists, its summary is shown next to the toggle, attributed to the agent and marked "not yet approved".
8. **Authority document follow-up required** toggle, inside the basis fold.
9. A confirm button labeled with the chosen option ("Confirm: Top, collapsible"). Selecting an option without pressing confirm records nothing.
10. **Withdraw question** (requires a reason).

For a resolved question, **Revise decision** opens the same form as a supersede.

### 8.5 Acceptance drawer

Shows:

- the candidate's plain summary;
- the owner's acceptance conditions as a checklist with each reported outcome;
- the untested scope;
- the reporter's attribution;
- the sentence "Accepting approves this exact version; later changes need acceptance again".

Folds: per-check evidence, then Technical details (repositories, heads, spec and acceptance versions, candidate ID).

Actions: **Accept this version**, and **Comment in discussion**, which opens the Discussion tab composer. Commenting is not a rejection. Stale candidates and already-accepted versions show status text instead of the button.

### 8.6 Room settings drawer

Sections:

- **Door**: repository references, the enrollment state with **Open the door** when enrollment is closed (resumable partial setup), and the join instructions with **Copy**.
- **Members**: participants with a short identifier, sessions with status and last synchronization, **Issue recovery file**, and **Revoke**. Revoke is danger-styled with confirmation text explaining that responsibility must be handed off separately.
- **Integrations**: the guided GitHub and DingTalk flows, the advanced binding and credential management, and delivery job status with retry or reconcile. The DingTalk flow states that notifications use the owner language (§11.3) and links to `/settings` to change it.

### 8.7 System settings

`/settings` is a full page with these sections:

- Language: Follow browser, or one of the four. The choice is the owner language (§11.2) and also applies to the activation page, tray and notifications. It is a write, so it is disabled while read-only.
- Theme: Follow system, Light, or Dark.
- Backup: consistent snapshot and readable export, showing the private paths.
- Storage: database size against the cap, with the 75% warning, and the command counters.
- Restore: instructions only. Restore stays an admin command with the service stopped.
- About: version and build fingerprint (from the build-info read in §16.1).

The duplicate "advanced project registration" form is dropped; `/rooms/new` with its multiple-repository fold covers it.

## 9. First use and activation

- **Activation page**: the one-time capability form that the service writes as a private local file and the launcher opens. Same flow and security, and the same file CSP (`default-src 'none'; style-src 'unsafe-inline'`), which allows its inline blueprint styles. Because no HTTP request precedes it, its language comes from the owner language and falls back to the OS UI language, then English (§11.3).
- **Expired page**: an HTTP response under the router's `default-src 'self'` policy. Its styles come from a same-origin stylesheet in the embedded UI assets, so the CSP does not change. Its language comes from the owner language, then `Accept-Language`, then English.
- **Not-activated state** in the SPA: a blueprint card with the tray instructions and **Check again**.
- **`/rooms/new`**, on one screen:
  1. **Repository path or URL**. The room name is filled in automatically from the last path segment and stays editable. A multiple-repositories fold holds up to 20 references.
  2. The consent sentence that opening the door allows agents of this OS user on this machine to join.
  3. **Open the door** runs `project.create` and then `enrollment.enable`, keeping today's unknown-result handling: the same request identity on retry, and no duplicate project.
  4. Join instructions appear on the same screen with **Copy** and **Go to the room**. A live "Waiting for the first agent to sit down…" line updates when the room's participant list becomes non-empty, refreshed on workspace events, and announces the change politely to screen readers. The line repeats that Roooooom does not start or wake agents.
- Integrations never appear in first use.
- A registered project with closed enrollment shows in the room tabs. Its room shows a banner offering **Open the door**. Setup is never duplicated (UX01).
- The join instructions are written in the current UI language. Paths and identifiers stay verbatim.

## 10. Visual system

The refined blueprint revision, approved by the owner on 2026-10-29 from an interactive sketch in local evidence, replaces the first palette and adds explicit scales. The blueprint metaphor stays: grid paper on the floor, ink rules, stamps and per-agent colors.

### 10.1 Palette

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| paper | `#f3efe4` | `#0c1626` | page and floor |
| paper-2 | `#e9e3d3` | `#111f35` | top bar, composer, drawer footer |
| sheet | `#fbf8f1` | `#15253d` | tables, panels, drawers and popovers |
| ink | `#13284a` | `#dce7f6` | text, rules, primary buttons |
| ink-muted | `#4b5b76` | `#9fb1ca` | secondary text (solid, AA on paper and sheet) |
| ink-2 | ink at 35% | ink at 33% | hairlines and inactive segments |
| grid | ink at 6% | ink at 5% | floor grid only |
| active | `#356a8f` | `#7fb3d5` | in-progress segments |
| owner (warning) | `#c2410c` | `#f59e0b` | **only** items that need the owner |
| on-owner | `#ffffff` | `#1b1203` | text on owner fills |
| owner-bg | `#fcefe4` | `#2a1d08` | owner item backgrounds |
| danger | `#9f1239` | `#fb7185` | cancel, revoke and other destructive actions |
| on-danger | `#ffffff` | `#2b0a13` | text on danger fills |
| note | `#fff4cf` | `#2e2915` | door notes |
| fresh | `#e3edf7` | `#1d3555` | a just-arrived entry, fading out |

Agent colors, light and dark: teal `#0f766e`/`#2dd4bf`, indigo `#4338ca`/`#a5b4fc`, fuchsia `#a21caf`/`#f0abfc`, olive `#4d7c0f`/`#bef264`, cerulean `#0369a1`/`#7dd3fc`, violet `#6d28d9`/`#c4b5fd`.

- A participant's color comes deterministically from its participant ID, so it is stable across sessions and browsers.
- Collisions are expected and acceptable: initials and same-name identifiers remain the identity.
- Agent colors never use the owner or danger hues.
- The builder may adjust lightness only to reach WCAG AA (4.5:1 for text on chips, 3:1 for non-text marks). Hue and purpose stay.
- Every text pair meets WCAG AA in both themes. A browser test measures the owner button, count badges, labels, secondary text and danger buttons in light and dark mode.

### 10.2 Typography

System fonts only; no embedded font files.

| Locale | Body stack |
| --- | --- |
| en | `"Segoe UI", system-ui, sans-serif` |
| zh-TW | `"Microsoft JhengHei", "PingFang TC", "Noto Sans TC", sans-serif` |
| zh-CN | `"Microsoft YaHei", "PingFang SC", "Noto Sans SC", sans-serif` |
| ja | `"Yu Gothic UI", "Hiragino Sans", "Noto Sans JP", sans-serif` |

- One type scale, nothing else: 26/700 page title, 20/700 drawer title, 17/700 table title and section lead, 15 body and buttons, 13 secondary text and feed, 12 mono times, counts and identifiers, 11 mono uppercase labels and stamps.
- Labels, times, counts and statuses use `"Cascadia Mono", "JetBrains Mono", Consolas, ui-monospace` in uppercase Latin. CJK labels use the body stack, never monospace or uppercase.
- `<html lang>` always matches the UI locale, so the browser picks the correct regional Han glyphs.
- Agent- or owner-authored blocks get `lang` only when the language is known; otherwise they inherit.

### 10.3 Space, rules and layers

- Spacing steps: 4, 8, 12, 16, 24 and 32 px.
- Rules: 1.5 px ink for primary lines, 1 px ink-2 for separators. Dashed outlines mean empty, provisional or stale. A double rule marks the door.
- Layers: grid paper only on the floor; tables, notes and panels are sheets with a 3 px hard offset shadow; drawers and popovers are sheets with a 6 px shadow. Hovering a table, note or button lifts it 2 px with a 5 px shadow; pressing sinks it 1 px.
- Corners are 2 px; seats and filter chips are pills.
- Stamps keep one size everywhere (11 px mono, 2 px border). On a table edge they tilt by −1.5°; in lists and drawer headers they sit level.
- Focus: a visible ink outline with offset.
- Icons: a small inline-SVG stroke set matching the rule weight; no icon font.
- Wordmark: `ROOOOOOM` in the mono stack, plus a simple door or floor-plan mark. Both are the builder's design within these rules.

### 10.4 Motion

| Step | Duration | Use |
| --- | --- | --- |
| quick | 120 ms | hover, press, color changes and every exit |
| base | 180 ms | drawer and popover entrances, content crossfades |
| shape | 280 ms | note-to-table morph, floor reorder, seat joins and leaves, tab indicator, stamp landing, room column resize |
| fill | 440 ms | progress segments |
| ink | 1.6 s | the highlight on a just-arrived feed entry as it fades |

- Entrances ease out; exits are shorter and ease in; stamps land with a slight overshoot. Count badges pop when their number changes. Loading shows dashed placeholder tables with a slow scan.
- The morph, reorder, seat changes and tab or view switches use same-document view transitions; a browser without them applies the change instantly.
- With `prefers-reduced-motion`, every transition, animation and view transition is off, and state still updates immediately.

### 10.5 Stamps

Stamps always pair text with an outline, so color never carries meaning alone.

| Stamp | Style |
| --- | --- |
| ⚖ Formal question | owner color, 2 px solid |
| ✓ Ready for your acceptance | ink, 2 px solid |
| ⚠ Needs your action | owner color, double rule |
| Offline import | ink, dashed |
| Destructive action | danger fill |

## 11. Language and copy

### 11.1 Copy rules

- Lead with what happened or what is needed, in the owner's terms. Identifiers, versions, epochs, SHAs and receipt states appear only in Technical details or folds.
- Never imply live presence. Write "Last synced 2 min ago", "Session ended", or "Waiting for an agent to take this".
- Label formal versus chat explicitly wherever both could be confused.
- Errors use the existing pattern: a plain localized sentence plus a diagnostic-code fold.
- Agent-authored text (summaries, questions, options, recommendations) appears verbatim with author attribution and is never machine-translated.

### 11.2 Web i18n

- Locales: `en`, `zh-CN`, `zh-TW`, `ja`. The fallback is `en`.
- Browser detection uses `navigator.languages`:
  - `zh-TW`, `zh-HK`, `zh-MO` and `zh-Hant*` map to `zh-TW`;
  - other `zh` variants map to `zh-CN`;
  - `ja*` maps to `ja`;
  - anything else maps to `en`.
- A manual choice is the **owner language**: stored on the server through the owner-only command in §16.1 and read with the owner context. Precedence in the Web UI is owner language, then browser detection. The UI may cache the last value in `localStorage` only to avoid a flash of the wrong language before the first read. It never writes the preference automatically.
- Catalogs are TypeScript modules with `en` as the source type. Other locales must satisfy the same key set at compile time.
  - Interpolation uses `{name}` placeholders through a typed `t()`.
  - Plurals use `Intl.PluralRules`, dates `Intl.DateTimeFormat`, relative times `Intl.RelativeTimeFormat`, and lists `Intl.ListFormat`.
- A verify step fails on catalog key mismatches and on CJK literals in `web/src` outside catalog files. Test fixtures that deliberately contain multilingual content are exempt.
- Layout allows about 35% English expansion over CJK. Japanese uses `line-break: strict`. Truncated text exposes its full value.

### 11.3 Server-side text

- Server-rendered text uses one resolution order: owner language, then a request signal where one exists (`Accept-Language` for the expired page), then the OS UI language of the host, then `en`. The mapping is the same as §11.2.
- Each surface:
  - The activation page uses the owner language, then the OS UI language, then English.
  - The expired page uses the owner language, then `Accept-Language`, then English.
  - The Windows tray uses the owner language when the service can provide it, otherwise the OS UI language. It refreshes its labels on its next menu open.
  - DingTalk notification text and its fixed footer use the owner language, then the OS UI language, then English. A language change affects only notifications created afterwards; queued jobs keep their text.
- The owner language is stored in the existing `meta` key-value table, so storing it needs no DDL.
- System-generated formal questions (today, the high-risk start approval in `internal/coordinator/decisions.go`) gain an optional additive `messageKey` (plus parameters) in their stored payload and the OpenAPI view. The UI renders the localized template when the key is known and falls back to the stored text otherwise. Agent-authored questions are untouched. Protocol version stays 1.
- Go catalogs are plain maps per locale with a test that enforces key parity.

## 12. Accessibility

- The door, floor, cabinet and lobby are lists (`role="list"`) of focusable items. Arrow keys move through tables in visual order (roving tabindex), and Enter opens the drawer.
- Each table's accessible name includes the goal, the phase, owner flags, and responsible seats with their last synchronization, for example "Article table of contents, in progress, waiting for your decision, integrating: Codex, last synced 2 minutes ago".
- The Needs-you control is a button with `aria-expanded` that controls a popover list of links.
- Live regions announce the first agent joining and changes in the Needs-you count, at most once per 10 seconds.
- Contrast meets WCAG AA. Color is never the only signal: stamps carry text, seats carry initials, and stale seats carry text.
- Primary controls keep at least a 44 px target; existing inputs and buttons keep 48 px.

## 13. Responsive behavior

- **1200 px and wider**: floor and activity sit side by side, and the floor fills the window height. Opening a drawer widens the right column to the drawer (540 px) in place of the activity column and folds the cabinet and lobby column, so the floor reflows instead of being covered.
- **760–1199 px**: a Floor | Activity switch replaces the side column. The drawer overlays about 70% of the width.
- **Below 760 px**: everything stacks.
  - Room tabs become a horizontal scroller.
  - Door notes become a horizontal strip.
  - Tables become a full-width list in priority order.
  - The cabinet and lobby become rows.
  - The drawer and the Needs-you list are full-screen.
- At no width does the page scroll horizontally (UX09). Long and multilingual text wraps.

## 14. States

- **Loading**: dashed skeleton tables.
- **Offline or read-only**: a top banner with the last synchronization time and **Reconnect**. Every write control is disabled with a reason, and the floor stays visible from the in-memory snapshot.
- **Re-synchronizing**: an indicator in the top bar.
- **Reset required**: a blocking banner with explicit recovery.
- **Authorization lost**: back to the not-activated card.
- **Stale form**: disabled, with Reopen.
- **Unknown write result**: the payload is kept and offered for explicit retry.
- **Empty room, empty Needs you, and empty archive**: each has its own explanatory state.

## 15. Preserved capabilities

Every current capability keeps a home:

| Current | New location |
| --- | --- |
| One-field request with optional requirements and unknown-result retry | Room composer, "Ask agents to do something" |
| Full task-create form | Composer → "I already have a complete specification" (drawer) |
| Project filter | Room tabs |
| Participant filter | Member focus (§7.2) |
| "Attention only" filter | Needs you |
| "Show cancelled and history" | File cabinet → archive |
| Project summary counts | Tab counts and floor |
| Members and recent sync | Lobby, seats, room settings → Members |
| Request detail and management | Request drawer |
| Task detail, specification and diagnostics | Task drawer, Overview and Technical details |
| Assignments, add and cancel | Task drawer → Work |
| Forced integrator handoff | Task drawer → Manage |
| Collaborator limit, cancel and reopen | Task drawer → Manage |
| Acceptance rule editor | Task drawer → Manage; linked from acceptance drawer |
| Deliveries, integrations, candidates, completions | Task drawer → Evidence |
| Formal answer, withdraw, revise, spec amendment, external authorization, follow-up | Formal question drawer |
| Human acceptance | Acceptance drawer |
| Owner blockers | Needs you plus table flags |
| Project and task chat, purpose, recipients, action required, receipts | Activity composer, task Discussion tab |
| Enrollment enable and join instructions | `/rooms/new`; room settings → Door |
| Participant recovery file and revoke; sessions list | Room settings → Members |
| GitHub and DingTalk guided and advanced setup, credential rotation, disable, job retry and reconcile | Room settings → Integrations |
| Backup, export, storage metrics, restore guidance | `/settings` |
| GitHub record links | Task drawer → Overview |

## 16. Server changes

### 16.1 Allowed additions

All additions are additive in `api/openapi.yaml` and the generated types, owner-scoped unless stated, covered by contract tests, and leave existing fields and semantics unchanged. Protocol version stays 1.

- **SPA routes**: extend the embedded UI route allowlist for `/rooms/...` and the legacy redirects, and point the post-activation redirect at `/`.
- **Times**: task `createdAt`, `updatedAt` and `completedAt`; project `createdAt`; and, for each owner blocker, the time it began. Derive these from the existing `events` table, which records `received_at` and is never pruned, when that is reliable and fast enough. Otherwise use §16.3.
- **Room reads** to avoid one request per table: assignments for all open tasks of a project in one query, and a Needs-you aggregate that returns pending questions, ready human acceptances (with staleness and the owner's existing acceptance already applied) and blockers across rooms, with their times.
- **Activity**: an optional read-only `activity` query per project (§7.5).
- **Build info**: version and build fingerprint, added to the owner context or exposed as a small read.
- **Owner language**: an owner-only command that sets or clears it, plus a read in the owner context (§11.2). It uses the standard command envelope and request identity. Agents cannot read or write it.
- **`messageKey`**: an optional key plus parameters on system-generated formal questions (§11.3).

### 16.2 Not allowed

- New authority, new agent capabilities, or changed meaning of any existing field, state or receipt.
- Server-side translation of stored owner or agent content.
- Any change to the router CSP or the activation-file CSP.

### 16.3 Schema migration (high risk)

A forward migration from durable schema 2 to 3 is allowed only when an addition in §16.1 cannot be derived reliably or efficiently from existing data. It follows DESIGN.md §12 and these rules:

- It runs in the existing single-writer, single-transaction migration path: foreign keys are checked before commit, and a failure leaves the database unchanged.
- It preserves service identity, generation and all history. Backfilled values come from `events.received_at`.
- It ships with migration tests from a schema-2 fixture, covering integrity and foreign-key checks, idempotent reopen, and rejection by schema-2 binaries.
- Recovery plan: the packaging upgrade notes require a consistent snapshot (the existing backup command) before the first start of the new binary. A schema-3 database cannot be opened by an older binary. Rolling back means stopping the service and restoring that snapshot, which loses writes made after the upgrade.
- DESIGN.md §12 and the packaging upgrade notes record the new schema version in the same change.

The builder records whether a migration was needed and why in the implementation plan.

## 17. Left to the builder

- Component structure, state handling and routing implementation.
- Exact spacing, sizes, icon drawings and the wordmark, within §10.
- Final copy in each catalog, within §11.
- Whether the activity feed needs a new read-only query (§7.5), and whether §16.3 is needed at all.
- Exact names and shapes of the §16.1 additions, within its rules.
- How "Last synced" stays fresh between events (for example a light periodic sessions read), since a synchronization alone may emit no event.
- The order of implementation steps.
- The cutover is a single switch on one branch. No old/new toggle ships, and the old `features/` code is removed at cutover.

## 18. Done means

- `npm --prefix web run verify` passes on the exact source: API types, typecheck, unit tests, build and embed, Go tests and vet, and Chromium end-to-end tests.
  - End-to-end selectors become locale-agnostic (roles plus accessible names from catalog lookups, or test IDs) and run under a pinned locale.
  - New end-to-end coverage: first use through join instructions; request to door note; conversion to table; answering a question from Needs you including Next item; acceptance; stale seat; member focus; archive search; offline read-only; stale-form reopen; keyboard floor navigation; narrow viewport without horizontal scroll; legacy redirects.
  - A smoke test renders the shell, room, each drawer type and first use in all four locales, checking for missing keys and overflow at narrow width.
- Go tests cover locale resolution for the activation and expired pages, the tray label locale, DingTalk text by owner language, the owner language command (owner-only, agents rejected), `messageKey` rendering fallback, every §16.1 read, catalog parity, and §16.3 migration tests if a migration exists.
- `node scripts/check-skill-contract.mjs` still passes, because the CLI is unchanged.
- Real-browser screenshots exist for the room, the Needs-you list, the formal question, acceptance, first use, and narrow layout, in light and dark and in all four locales, for owner review.
- DESIGN.md §11 is rewritten to the room model, the daily-workflow paragraphs of the four READMEs are updated, this document's status changes to implemented or is folded into DESIGN.md, and the publication and documentation gates pass.
