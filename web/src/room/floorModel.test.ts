import {describe,expect,it} from 'vitest';
import {buildFloor,COMPACT_AFTER,STALE_AFTER_MS,type FloorInput} from './floorModel';

const now=new Date('2026-10-29T12:00:00Z');
const ago=(ms:number)=>new Date(now.getTime()-ms).toISOString();
type Task=FloorInput['tasks'][number];
type Assignment=FloorInput['assignments'][number];
const task=(id:string,patch:Partial<Task>={}):Task=>({id,projectId:'p',goal:'Goal '+id,phase:'active',integratorId:'cx',integratorSessionId:'s-cx',canStart:true,attention:[],collaboratorLimit:2,collaboratorCount:0,createdAt:ago(3_600_000),updatedAt:ago(60_000),...patch} as Task);
const assignment=(id:string,taskId:string,patch:Partial<Assignment>={}):Assignment=>({id,taskId,role:'core',state:'claimed',claimParticipantId:'cc',claimSessionId:'s-cc',canStart:true,...patch} as Assignment);
const participants=[{id:'cx',name:'Codex'},{id:'cc',name:'Claude Code'},{id:'gm',name:'Gemini CLI'}];
const sessions=[
 {id:'s-cx',participantId:'cx',status:'active',lastSeenAt:ago(120_000)},
 {id:'s-cc',participantId:'cc',status:'active',lastSeenAt:ago(9*60_000)},
 {id:'s-gm',participantId:'gm',status:'active',lastSeenAt:ago(3_600_000)},
] as FloorInput['sessions'];
const input=(patch:Partial<FloorInput>={}):FloorInput=>({tasks:[],requests:[],assignments:[],participants,sessions,needsYou:[],now,...patch});

describe('buildFloor',()=>{
 it('places open tasks on the floor, done tasks in the cabinet and drops cancelled ones',()=>{
  const floor=buildFloor(input({tasks:[task('a'),task('b',{phase:'ready',integratorId:undefined}),task('c',{phase:'done',completedAt:ago(1000)}),task('d',{phase:'cancelled'})]}));
  expect(floor.tables.map(t=>t.taskId).sort()).toEqual(['a','b']);
  expect(floor.cabinet.doneCount).toBe(1);
  expect(floor.cabinet.recent).toEqual([{taskId:'c',goal:'Goal c'}]);
 });
 it('gives one seat per participant per table',()=>{
  const floor=buildFloor(input({tasks:[task('a')],assignments:[assignment('x1','a'),assignment('x2','a',{role:'web'})]}));
  const seats=floor.tables[0].seats;
  expect(seats.map(s=>s.participantId)).toEqual(['cx','cc']);
  expect(seats[0].role).toBe('integrating');
  expect(seats[1].role).toBe('core');
 });
 it('marks a worker delivered when all their assignments are submitted or accepted',()=>{
  const floor=buildFloor(input({tasks:[task('a')],assignments:[assignment('x1','a',{state:'submitted'}),assignment('x2','a',{state:'accepted'})]}));
  expect(floor.tables[0].seats[1].role).toBe('delivered');
 });
 it('applies the stale boundary exactly at 15 minutes',()=>{
  const at=(ms:number)=>buildFloor(input({tasks:[task('a')],sessions:[{...sessions[0],lastSeenAt:ago(ms)}]})).tables[0].seats[0].stale;
  expect(STALE_AFTER_MS).toBe(15*60_000);
  expect(at(STALE_AFTER_MS)).toBe(false);
  expect(at(STALE_AFTER_MS+1)).toBe(true);
 });
 it('reports ended sessions and revoked participants',()=>{
  const ended=buildFloor(input({tasks:[task('a')],sessions:[{...sessions[0],status:'ended'}]})).tables[0].seats[0];
  expect(ended.ended).toBe(true);
  const revoked=buildFloor(input({tasks:[task('a',{integratorId:'gone'})]})).tables[0].seats[0];
  expect(revoked).toMatchObject({participantId:'gone',revoked:true,showShortId:true});
 });
 it('disambiguates same-name agents with a short identifier',()=>{
  const twins=[{id:'aaaaaaaa1',name:'Agent'},{id:'bbbbbbbb2',name:'Agent'}];
  const floor=buildFloor(input({participants:twins,tasks:[task('a',{integratorId:'aaaaaaaa1'})],sessions:[]}));
  expect(floor.tables[0].seats[0]).toMatchObject({name:'Agent',shortId:'aaaaaaaa',showShortId:true});
  expect(floor.lobby[0]).toMatchObject({participantId:'bbbbbbbb2',showShortId:true});
 });
 it('switches to compact tables above eight open tasks',()=>{
  const tasks=Array.from({length:COMPACT_AFTER+1},(_,i)=>task('t'+i));
  expect(buildFloor(input({tasks})).density).toBe('compact');
  expect(buildFloor(input({tasks:tasks.slice(0,COMPACT_AFTER)})).density).toBe('comfortable');
 });
 it('orders flagged tables first, then active by recent activity, then ready by age',()=>{
  const tasks=[task('old',{updatedAt:ago(900_000)}),task('new',{updatedAt:ago(1000)}),task('flag',{updatedAt:ago(9_000_000)}),task('ready2',{phase:'ready',createdAt:ago(1000)}),task('ready1',{phase:'ready',createdAt:ago(9000)})];
  const needsYou=[{kind:'decision',projectId:'p',taskId:'flag',summary:'q',since:ago(1)}] as FloorInput['needsYou'];
  expect(buildFloor(input({tasks,needsYou})).tables.map(t=>t.taskId)).toEqual(['flag','new','old','ready1','ready2']);
 });
 it('maps owner items to table flags',()=>{
  const needsYou=[{kind:'acceptance',projectId:'p',taskId:'a',summary:'',since:ago(1)},{kind:'blocker',projectId:'p',taskId:'a',code:'integrator-revoked',summary:'',since:ago(1)}] as FloorInput['needsYou'];
  expect(buildFloor(input({tasks:[task('a')],needsYou})).tables[0].flags).toEqual(['acceptance','blocker']);
 });
 it('derives segments from assignment states',()=>{
  const floor=buildFloor(input({tasks:[task('a')],assignments:[assignment('1','a',{state:'accepted'}),assignment('2','a',{state:'active'}),assignment('3','a',{state:'open',claimParticipantId:undefined}),assignment('4','a',{state:'claimed',canStart:false}),assignment('5','a',{state:'cancelled'})]}));
  expect(floor.tables[0].segments).toEqual(['accepted','active','open','blocked']);
 });
 it('notes waiting and preparing requests at the door',()=>{
  const requests=[{id:'r1',projectId:'p',text:'RSS feed',state:'waiting'},{id:'r2',projectId:'p',text:'Spam guard',state:'preparing',preparerParticipantId:'cx',preparerSessionId:'s-cx'},{id:'r3',projectId:'p',text:'done',state:'converted'}] as FloorInput['requests'];
  const needsYou=[{kind:'decision',projectId:'p',requestId:'r2',summary:'q',since:ago(1)}] as FloorInput['needsYou'];
  const floor=buildFloor(input({requests,needsYou}));
  expect(floor.notes.map(n=>[n.requestId,n.state,n.preparer?.participantId,n.flagged])).toEqual([['r1','waiting',undefined,false],['r2','preparing','cx',true]]);
 });
 it('keeps members with open responsibility out of the lobby',()=>{
  const floor=buildFloor(input({tasks:[task('a')],assignments:[assignment('x','a')]}));
  expect(floor.lobby.map(s=>s.participantId)).toEqual(['gm']);
  expect(floor.lobby[0].stale).toBe(true);
 });
 it('handles a large room quickly',()=>{
  const tasks=[...Array.from({length:30},(_,i)=>task('o'+i)),...Array.from({length:200},(_,i)=>task('d'+i,{phase:'done',completedAt:ago(i*1000)}))];
  const started=performance.now();
  const floor=buildFloor(input({tasks}));
  expect(performance.now()-started).toBeLessThan(50);
  expect(floor).toMatchObject({density:'compact',cabinet:{doneCount:200}});
  expect(floor.tables).toHaveLength(30);
  expect(floor.cabinet.recent.map(r=>r.taskId)).toEqual(['d0','d1','d2']);
 });
});
