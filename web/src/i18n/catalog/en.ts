// Source catalog: every other locale must provide exactly these keys.
export const en={
 'app.name':'Roooooom',
 'time.lastSynced':'Last synced {time}',
 'time.never':'Not yet observed',
 'count.agents.one':'{count} agent',
 'count.agents.other':'{count} agents',
 'error.offline':'The service is unreachable. Your input is kept; check the current state after it recovers, then retry explicitly.',
 'error.invalid':'Check the input and requirements, then submit again.',
 'error.forbidden':'This action needs the matching permission. Check the current identity.',
 'error.github-metadata-unavailable':'GitHub cannot be reached right now. The credential is kept; confirm the destination later.',
 'error.github-metadata-denied-or-moved':'GitHub did not allow reading this destination. Check the URL, token scope and repository access.',
 'error.github-repository-unavailable':'This repository cannot hold Issue records. Make sure Issues are enabled and it is not archived.',
 'error.read-only-until-synchronized':'Reconnecting. Actions become available after synchronization.',
 'error.unknown':'The action is not confirmed. Reload the current state and check it before retrying.',
 'feedback.diagnostics':'Diagnostic details',
 'fold.technical':'Technical details',
 'stamp.formal':'Formal question',
 'stamp.acceptance':'Ready for your acceptance',
 'stamp.blocker':'Needs your action',
 'stamp.offline':'Offline import',
} as const;
export type MessageKey=keyof typeof en;
