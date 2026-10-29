import {describe,expect,it} from 'vitest';
import {approveAlso} from './approveAlso';

type Q=Parameters<typeof approveAlso>[0];
type T=NonNullable<Parameters<typeof approveAlso>[1]>;
const task={id:'t',goal:'Task goal',scope:['a'],nonScope:['b'],authorityRefs:['ref'],risk:'normal'} as T;
const question=(patch:Partial<Q>):Q=>({id:'q',blockedActions:[],...patch} as Q);

describe('approveAlso',()=>{
 it('offers a spec amendment from the task when the question blocks spec changes without a proposal',()=>{
  expect(approveAlso(question({blockedActions:['implementation','spec-change']}),task)).toEqual({spec:{prefill:{goal:'Task goal',scope:['a'],nonScope:['b'],authorityRefs:['ref'],risk:'normal'},proposed:false}});
 });
 it('prefills from the agent proposal and marks it proposed',()=>{
  const specProposal={goal:'Proposed',scope:['x'],nonScope:[],authorityRefs:['r2'],risk:'high'};
  expect(approveAlso(question({blockedActions:['spec-change'],specProposal} as Partial<Q>),task).spec).toEqual({prefill:specProposal,proposed:true});
 });
 it('needs a task for a spec amendment',()=>{
  expect(approveAlso(question({blockedActions:['spec-change']}),undefined)).toEqual({});
 });
 it('offers external authorization only when the question blocks external actions',()=>{
  const externalProposal={action:'push',target:'origin/x',scope:'one branch',originalText:'push once',expiresAt:'2026-10-30T00:00:00Z',maxUses:1};
  expect(approveAlso(question({blockedActions:['external-action'],externalProposal} as Partial<Q>),task)).toEqual({external:{prefill:externalProposal,proposed:true}});
  expect(approveAlso(question({blockedActions:['external-action']}),task)).toEqual({external:{prefill:{},proposed:false}});
 });
 it('offers nothing for an ordinary question',()=>{
  expect(approveAlso(question({blockedActions:['implementation']}),task)).toEqual({});
 });
});
