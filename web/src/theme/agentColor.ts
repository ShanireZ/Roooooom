export type AgentColor=0|1|2|3|4|5;
// 32-bit FNV-1a keeps a participant's color stable across sessions and browsers.
// Collisions are expected; initials and short identifiers remain the identity.
export function agentColor(participantId:string):AgentColor{
 let hash=0x811c9dc5;
 for(let i=0;i<participantId.length;i++){hash^=participantId.charCodeAt(i);hash=Math.imul(hash,0x01000193)>>>0}
 return (hash%6) as AgentColor;
}
