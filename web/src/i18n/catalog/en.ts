// Source catalog: every other locale must provide exactly these keys.
export const en={
 'app.name':'Roooooom',
 'time.lastSynced':'Last synced {time}',
 'time.never':'Not yet observed',
 'count.agents.one':'{count} agent',
 'count.agents.other':'{count} agents',
} as const;
export type MessageKey=keyof typeof en;
