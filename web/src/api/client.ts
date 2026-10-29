import type {components} from './contracts.generated';
export type Schema=components['schemas'];
export type Envelope<T>=Omit<Schema['Envelope'],'result'> & {result:T};
export type Identity=Schema['Identity'];
export type OwnerContext={identity:Identity;csrf:string;actorKind:string;ownerLanguage:string;build:Record<string,unknown>};
export class ApiError extends Error{
 constructor(public code:string,public status:number){super(code)}
}
let owner:OwnerContext|undefined;
let writesEnabled=false;
const failures=new Set<(error:ApiError)=>void>();
export function subscribeFailures(listener:(error:ApiError)=>void){failures.add(listener);return()=>{failures.delete(listener)}}
function failed(error:ApiError){permitWrites(false);for(const listener of failures)listener(error);return error}
export function permitWrites(value:boolean){writesEnabled=value}
export function ownerContext(){return owner}
export function clearOwner(){owner=undefined;writesEnabled=false}
// Explicit retries in this open page reuse the exact request. This is volatile
// memory, never a browser outbox or an automatic mutation replay mechanism.
const pending=new Map<string,string>();
function ordered(value:unknown):unknown{
 if(Array.isArray(value))return value.map(ordered);
 if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).sort(([a],[b])=>a.localeCompare(b)).map(([k,v])=>[k,ordered(v)]));
 return value;
}
function checkedPath(path:string){
 if(!path.startsWith('/api/v1/')||path.startsWith('//')||path.includes('\\')||path.includes('#')||/%2e|%2f|%5c/i.test(path)||path.split('?')[0].split('/').some(x=>x==='.'||x==='..')){
  throw new ApiError('invalid-local-api-path',400);
 }
}
export async function apiRequest<T>(path:string,init:RequestInit={}):Promise<Envelope<T>>{
 checkedPath(path);
 const method=(init.method??'GET').toUpperCase();
 const headers=new Headers(init.headers);
 if(method!=='GET'){
  if(!owner || !writesEnabled)throw new ApiError('read-only-until-synchronized',503);
  headers.set('X-CSRF-Token',owner.csrf);
  headers.set('Content-Type','application/json');
 }
 headers.set('Accept','application/json');
 let response:Response;
 try{response=await fetch(path,{...init,method,headers,credentials:'same-origin',redirect:'error',cache:'no-store'})}
 catch{throw failed(new ApiError('offline',503))}
 let data:unknown;
 try{data=await response.json()}catch{throw failed(new ApiError('invalid-response',502))}
 if(!response.ok){
  const code=(data as {error?:{code?:string}})?.error?.code??'request-rejected';
  const error=new ApiError(code,response.status);
  if(response.status===401){clearOwner();throw failed(error)}
  if(response.status>=500)throw failed(error);
  throw error;
 }
 if(!data||typeof data!=='object'||!('result' in data))throw failed(new ApiError('invalid-envelope',502));
 return data as Envelope<T>;
}
export async function initializeOwner():Promise<OwnerContext>{
 const response=await fetch('/identity',{credentials:'omit',redirect:'error',cache:'no-store'});
 const identity=await response.json() as Identity;
 if(!response.ok||identity.status!=='ok'||identity.protocolVersion!==1)throw new ApiError('service-mismatch',409);
 if(owner&&owner.identity.serviceId!==identity.serviceId){clearOwner();throw new ApiError('service-mismatch',409)}
 const me=await apiRequest<{actorKind:string;csrf?:string;ownerLanguage?:string;build?:Record<string,unknown>}>('/api/v1/me');
 if(me.result.actorKind!=='human'||!me.result.csrf){clearOwner();throw new ApiError('owner-required',403)}
 owner={identity,csrf:me.result.csrf,actorKind:'human',ownerLanguage:me.result.ownerLanguage??'',build:me.result.build??{}};
 return owner;
}
export async function command<T>(kind:string,payload:unknown,targetId?:string,versions:Partial<Schema['Mutation']>={}):Promise<Envelope<T>>{
 if(!/^[a-z]+\.[a-z]+$/.test(kind))throw new ApiError('invalid-command',400);
 if(!owner)throw new ApiError('owner-required',401);
 const {clientRequestId:ignored,...applicable}=versions;
 const base={...applicable,serviceId:owner.identity.serviceId,datasetGeneration:owner.identity.datasetGeneration,targetId,payload};
 const key=kind+'|'+JSON.stringify(ordered(JSON.parse(JSON.stringify(base))));
 let body=pending.get(key);
 if(!body){
  if(pending.size>=20)throw new ApiError('unconfirmed-request-budget-exceeded',409);
  body=JSON.stringify({...base,clientRequestId:crypto.randomUUID()});pending.set(key,body);
 }
 try{
  const result=await apiRequest<T>('/api/v1/commands/'+kind,{method:'POST',body});
  pending.delete(key);return result;
 }catch(error){
  if(error instanceof ApiError&&error.status<500)pending.delete(key);
  throw error;
 }
}
