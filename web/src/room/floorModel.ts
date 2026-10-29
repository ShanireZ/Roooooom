import type {Schema} from '../api/client';
import type {Segment} from '../ui/ProgressBar';
import {agentColor,type AgentColor} from '../theme/agentColor';

export const STALE_AFTER_MS=15*60*1000;
export const COMPACT_AFTER=8;
const OPEN=['ready','active','integrating','review'];

export type FloorInput={
 tasks:Schema['Task'][];requests:Schema['WorkRequest'][];assignments:Schema['Assignment'][];
 participants:Schema['ParticipantView'][];sessions:Schema['SessionView'][];needsYou:Schema['NeedsYouItem'][];now:Date;
};
// `name` is empty for revoked participants; the view renders the catalog label.
export type Seat={participantId:string;name:string;shortId:string;showShortId:boolean;color:AgentColor;role:'integrating'|'delivered'|string;lastSeenAt:string;stale:boolean;ended:boolean;revoked:boolean};
export type TableFlag='decision'|'acceptance'|'blocker';
export type Table={taskId:string;goal:string;phase:string;segments:Segment[];seats:Seat[];flags:TableFlag[];waitingForAgent:boolean;waitingOnDependencies:boolean;updatedAt:string};
export type Note={requestId:string;text:string;state:'waiting'|'preparing';preparer?:Seat;flagged:boolean};
export type FloorModel={notes:Note[];tables:Table[];density:'comfortable'|'compact';cabinet:{doneCount:number;recent:{taskId:string;goal:string}[]};lobby:Seat[]};

const time=(iso?:string)=>iso&&!iso.startsWith('0001-')?new Date(iso).getTime():0;

export function buildFloor({tasks,requests,assignments,participants,sessions,needsYou,now}:FloorInput):FloorModel{
 const roster=new Map(participants.map(p=>[p.id,p]));
 const sessionById=new Map(sessions.map(s=>[s.id,s]));
 const nameCount=new Map<string,number>();
 for(const p of participants)nameCount.set(p.name,(nameCount.get(p.name)??0)+1);
 const seat=(participantId:string,sessionId:string|undefined,role:string):Seat=>{
  const person=roster.get(participantId),session=sessionId?sessionById.get(sessionId):undefined;
  const lastSeenAt=session?.lastSeenAt??'',seen=time(lastSeenAt);
  return {participantId,name:person?.name??'',shortId:participantId.slice(0,8),showShortId:!person||(nameCount.get(person.name)??0)>1,color:agentColor(participantId),role,lastSeenAt,
   stale:!seen||now.getTime()-seen>STALE_AFTER_MS,ended:session?.status==='ended',revoked:!person};
 };
 const flagsOf=(taskId:string)=>(['decision','acceptance','blocker'] as TableFlag[]).filter(kind=>needsYou.some(i=>i.kind===kind&&i.taskId===taskId));
 const byTask=new Map<string,Schema['Assignment'][]>();
 for(const a of assignments)if(a.state!=='cancelled')(byTask.get(a.taskId)??byTask.set(a.taskId,[]).get(a.taskId)!).push(a);
 const busy=new Set<string>();

 const tables:Table[]=tasks.filter(t=>OPEN.includes(t.phase)).map(t=>{
  const work=byTask.get(t.id)??[],flags=flagsOf(t.id);
  const seats:Seat[]=[];
  if(t.integratorId){seats.push(seat(t.integratorId,t.integratorSessionId,'integrating'));busy.add(t.integratorId)}
  for(const a of work){
   if(!a.claimParticipantId||seats.some(s=>s.participantId===a.claimParticipantId))continue;
   const mine=work.filter(x=>x.claimParticipantId===a.claimParticipantId);
   const delivered=mine.every(x=>x.state==='submitted'||x.state==='accepted');
   seats.push(seat(a.claimParticipantId,a.claimSessionId,delivered?'delivered':a.role));
   if(mine.some(x=>x.state!=='accepted'))busy.add(a.claimParticipantId);
  }
  const segments:Segment[]=work.length?work.map(a=>a.state==='accepted'?'accepted':a.state==='open'?'open':!a.canStart&&a.state!=='submitted'?(flags.includes('decision')?'owner':'blocked'):'active')
   :[flags.length?'owner':!t.integratorId?'open':'active'];
  return {taskId:t.id,goal:t.goal,phase:t.phase,segments,seats,flags,waitingForAgent:!t.integratorId,waitingOnDependencies:!!t.integratorId&&!t.canStart&&!flags.length,updatedAt:t.updatedAt??''};
 });
 const created=new Map(tasks.map(t=>[t.id,time(t.createdAt)]));
 const rank=(t:Table)=>t.flags.length?0:t.phase==='ready'?2:1;
 tables.sort((a,b)=>rank(a)-rank(b)||(rank(a)===2?(created.get(a.taskId)??0)-(created.get(b.taskId)??0):time(b.updatedAt)-time(a.updatedAt)));

 const notes:Note[]=requests.filter(r=>r.state==='waiting'||r.state==='preparing').map(r=>{
  if(r.preparerParticipantId)busy.add(r.preparerParticipantId);
  return {requestId:r.id,text:r.text,state:r.state as Note['state'],preparer:r.preparerParticipantId?seat(r.preparerParticipantId,r.preparerSessionId,'preparing'):undefined,
   flagged:needsYou.some(i=>i.kind==='decision'&&i.requestId===r.id&&!i.taskId)};
 });

 const done=tasks.filter(t=>t.phase==='done').sort((a,b)=>time(b.completedAt??b.updatedAt)-time(a.completedAt??a.updatedAt));
 const latestSession=(id:string)=>sessions.filter(s=>s.participantId===id).sort((a,b)=>time(b.lastSeenAt)-time(a.lastSeenAt))[0]?.id;
 const lobby=participants.filter(p=>!busy.has(p.id)).map(p=>seat(p.id,latestSession(p.id),''));
 return {notes,tables,density:tables.length>COMPACT_AFTER?'compact':'comfortable',cabinet:{doneCount:done.length,recent:done.slice(0,3).map(t=>({taskId:t.id,goal:t.goal}))},lobby};
}
