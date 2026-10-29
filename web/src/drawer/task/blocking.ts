import type {MessageKey} from '../../i18n/catalog/en';

// The plain-language reason for an attention code, for task and request blockers alike.
export function blockingKey(code:string):MessageKey|undefined{
 if(code.startsWith('decision-attention:'))return 'blocking.decision';
 if(code.startsWith('pending-reconciliation:'))return 'blocking.reconciliation';
 return ({'integrator-needed':'blocking.integratorNeeded','integrator-revoked':'blocking.integratorRevoked','integrator-session-ended':'blocking.sessionEnded','integrator-presence-stale':'blocking.presenceStale','owner-decision-needed':'blocking.ownerDecision',
  'preparer-revoked':'blocking.preparerRevoked','preparer-session-ended':'blocking.preparerSessionEnded'} as Record<string,MessageKey>)[code];
}
