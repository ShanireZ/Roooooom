import {describe,expect,it} from 'vitest';
import {ApiError} from '../api/client';
import {requestPayload,submitRequest,type RequestFormValues,type RequestSubmitState} from './requestSubmit';

const values=(patch:Partial<RequestFormValues>={}):RequestFormValues=>({projectId:'p',text:'Weekly summary',constraints:[],acceptanceConditions:[],acceptanceMode:'integrator',...patch});

describe('requestPayload',()=>{
 it('includes optional requirements only when set',()=>{
  expect(requestPayload(values())).toEqual({projectId:'p',text:'Weekly summary',constraints:[]});
  expect(requestPayload(values({constraints:['keep login'],collaboratorLimit:2,designatedIntegratorId:'cx',acceptanceConditions:['tests pass'],acceptanceMode:'human'})))
   .toEqual({projectId:'p',text:'Weekly summary',constraints:['keep login'],collaboratorLimit:2,designatedIntegratorId:'cx',acceptancePolicy:{mode:'human',conditions:['tests pass'],ownerProtected:true}});
 });
});

describe('submitRequest',()=>{
 it('retries an unknown result with the identical first payload even if the form changed',async()=>{
  const sent:unknown[]=[];let fail=true;
  const send=async(kind:string,payload:unknown)=>{sent.push(JSON.stringify([kind,payload]));if(fail){fail=false;throw new ApiError('offline',503)}return {result:{id:'r1'}} as never};
  const state:RequestSubmitState={};
  await expect(submitRequest(values({constraints:['keep login']}),state,send)).rejects.toThrow('offline');
  expect(state.submitted).toBeDefined();
  await submitRequest(values({text:'edited while disabled'}),state,send);
  expect(sent).toHaveLength(2);
  expect(sent[1]).toBe(sent[0]);
  expect(state.submitted).toBeUndefined();
 });
 it('clears the retained payload after a known rejection',async()=>{
  const state:RequestSubmitState={};
  await expect(submitRequest(values(),state,async()=>{throw new ApiError('invalid',400)})).rejects.toThrow('invalid');
  expect(state.submitted).toBeUndefined();
 });
});
