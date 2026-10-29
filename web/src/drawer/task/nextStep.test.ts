import {describe,expect,it} from 'vitest';
import {nextStep,type NextStepInput} from './nextStep';

type Task=NextStepInput['task'];
const task=(patch:Partial<Task>={}):Task=>({id:'t',phase:'active',integratorId:'cx',canStart:true,...patch} as Task);
const names={cx:'Codex'};

describe('nextStep',()=>{
 it('points at a pending decision first',()=>{
  const step=nextStep({task:task(),assignments:[],needsYou:[{kind:'decision',projectId:'p',taskId:'t',summary:'Where does the TOC go?',since:''}] as NextStepInput['needsYou'],names});
  expect(step).toEqual({key:'task.next.decision',params:{question:'Where does the TOC go?'}});
 });
 it('names a system question in the reader language',()=>{
  const needsYou=[{kind:'decision',projectId:'p',taskId:'t',messageKey:'system.riskStart',summary:'是否批准此高風險任務？',since:''}] as NextStepInput['needsYou'];
  const step=nextStep({task:task(),assignments:[],needsYou,names,summary:i=>i.messageKey==='system.riskStart'?'Approve this high-risk task?':i.summary});
  expect(step).toEqual({key:'task.next.decision',params:{question:'Approve this high-risk task?'}});
 });
 it('counts deliveries while integrating',()=>{
  const assignments=[{state:'submitted'},{state:'accepted'},{state:'claimed'}] as NextStepInput['assignments'];
  expect(nextStep({task:task({phase:'integrating'}),assignments,needsYou:[],names})).toEqual({key:'task.next.integrating',params:{name:'Codex',count:2}});
 });
 it('waits for an agent when nobody integrates',()=>{
  expect(nextStep({task:task({integratorId:undefined,phase:'ready'}),assignments:[],needsYou:[],names}).key).toBe('task.next.waitingAgent');
 });
 it('reports completion',()=>{
  expect(nextStep({task:task({phase:'done'}),assignments:[],needsYou:[],names}).key).toBe('task.next.done');
 });
 it('reports acceptance and blockers before progress',()=>{
  const needsYou=[{kind:'acceptance',projectId:'p',taskId:'t',summary:'',since:''}] as NextStepInput['needsYou'];
  expect(nextStep({task:task(),assignments:[],needsYou,names}).key).toBe('task.next.acceptance');
  expect(nextStep({task:task(),assignments:[],needsYou:[{...needsYou[0],kind:'blocker'}],names}).key).toBe('task.next.blocker');
 });
 it('names unmet dependencies without implying live work',()=>{
  expect(nextStep({task:task({canStart:false}),assignments:[],needsYou:[],names}).key).toBe('task.next.dependencies');
  expect(nextStep({task:task(),assignments:[],needsYou:[],names})).toEqual({key:'task.next.active',params:{name:'Codex'}});
 });
});
