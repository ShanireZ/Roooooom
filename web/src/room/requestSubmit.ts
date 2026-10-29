import {ApiError,command,type Envelope,type Schema} from '../api/client';

export type RequestFormValues={projectId:string;text:string;constraints:string[];collaboratorLimit?:number;designatedIntegratorId?:string;acceptanceConditions:string[];acceptanceMode:'integrator'|'human'};
export type RequestSubmitState={submitted?:Schema['RequestCreate']};
type Send=<T>(kind:string,payload:unknown)=>Promise<Envelope<T>>;

export function requestPayload(v:RequestFormValues):Schema['RequestCreate']{
 return {projectId:v.projectId,text:v.text,constraints:v.constraints,
  ...(v.collaboratorLimit!==undefined?{collaboratorLimit:v.collaboratorLimit}:{}),
  ...(v.designatedIntegratorId?{designatedIntegratorId:v.designatedIntegratorId}:{}),
  ...(v.acceptanceConditions.length?{acceptancePolicy:{mode:v.acceptanceMode,conditions:v.acceptanceConditions,ownerProtected:true}}:{})};
}
// Disabled optional fields vanish from a form on retry, so the first complete payload
// is kept until the result is known; the client then reuses its exact request identity.
export async function submitRequest(v:RequestFormValues,state:RequestSubmitState,send:Send=command):Promise<Envelope<Schema['WorkRequest']>>{
 state.submitted??=requestPayload(v);
 try{
  const result=await send<Schema['WorkRequest']>('request.create',state.submitted);
  state.submitted=undefined;
  return result;
 }catch(e){
  if(e instanceof ApiError&&e.status<500)state.submitted=undefined;
  throw e;
 }
}
