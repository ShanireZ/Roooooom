# Explicitly authorized external integrations

This is a procedure, not a passed report. See the [design](../../DESIGN.md) and [evidence format](../../docs/evidence/acceptance-template.md).

Default tests use local fake providers and synthetic credentials. A real-provider run requires separately authorized exact repositories, bots, field/event scopes and permitted writes. Publishing this source repository does not grant the product access to live GitHub or DingTalk credentials.

GitHub setup stores a fine-grained repository-limited PAT privately, inspects only official metadata, then presents the canonical destination and content for explicit confirmation. Metadata success is not Issues-write proof. Verify one task maps to one Issue, selected milestones append, unknown responses reconcile without blind duplicates, human Issue edits are retained and no remote edit overwrites local task authority.

Exercise permission expiry, rate limits, redirect rejection, canonical-ID/public-private changes, credential rotation, target switch and restore. Old jobs keep their original binding and uncertainty; disabling a binding never silently retargets them. Record actual authorized provider results for A29, and fake/negative evidence for A27/A30.

DingTalk accepts only the official complete HTTPS Webhook URL plus signing secret. Save and enable must not send a test notification. Check response errcode as well as HTTP status, signature/time/rate errors, bounded retries, shared-bot capacity and stale-item suppression. Platform acceptance does not mean a phone or person read it; timeout can duplicate the same notification identity.

For authorized A33, observe the real device receiving a notification. Its localhost link cannot open the host machine's page from another device. Group replies cannot become formal decisions; use the host's decision page. Retain provider/device evidence privately without token, signing secret or unredacted URLs. See A29-A33 and UX07.
