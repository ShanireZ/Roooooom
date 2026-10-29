import {useEffect,useState} from 'react';
import {apiRequest,ApiError,ownerContext} from './client';
import {useWorkspaceEvents} from './events';
const snapshots=new Map<string,{data:unknown;revision:number}>();
const requests=new Map<string,Promise<unknown>>();
let cacheEpoch=0;
export function clearSnapshots(){cacheEpoch++;snapshots.clear();requests.clear()}
async function collection(path:string):Promise<unknown[]>{
 const url=new URL(path,window.location.origin);
 const result:unknown[]=[];let cursor='';
 for(let page=0;page<1000;page++){
  if(cursor)url.searchParams.set('cursor',cursor);
  const response=await apiRequest<unknown[]>(url.pathname+url.search);
  if(!Array.isArray(response.result))throw new ApiError('invalid-collection',502);
  result.push(...response.result);
  if(result.length>5000)throw new ApiError('collection-budget-exceeded-filter-project',429);
  if(!response.hasMore)return result;
  if(!response.cursor||response.cursor===cursor)throw new ApiError('invalid-page-cursor',502);
  cursor=response.cursor;
 }
 throw new ApiError('collection-budget-exceeded-filter-project',429);
}
export function useResource<T>(path:string,enabled=true,allPages=false){
 const event=useWorkspaceEvents();
 const identity=ownerContext()?.identity;
 const key=[cacheEpoch,identity?.serviceId,identity?.datasetGeneration,path,allPages].join('|');
 const [state,setState]=useState<{key:string;data?:T;error?:string;fresh:boolean}>({key,data:snapshots.get(key)?.data as T|undefined,fresh:false});
 const [refreshKey,setRefreshKey]=useState(0);
 useEffect(()=>{
  let current=true;
  setState({key,data:snapshots.get(key)?.data as T|undefined,fresh:false});
  if(!enabled || !event.connected)return ()=>{current=false};
  const requestKey=key+'|'+event.revision;
  let request=requests.get(requestKey);
  if(!request){
   request=allPages?collection(path):apiRequest<T>(path).then(response=>response.result);
   requests.set(requestKey,request);
   void request.finally(()=>{if(requests.get(requestKey)===request)requests.delete(requestKey)}).catch(()=>{});
  }
  request.then(data=>{
   const prior=snapshots.get(key);
   if(!prior||prior.revision<=event.revision)snapshots.set(key,{data,revision:event.revision});
   if(current)setState({key,data:data as T,fresh:true});
  }).catch((error:unknown)=>{
   if(current)setState(previous=>({...previous,error:error instanceof ApiError?error.code:'offline',fresh:false}));
  });
  return()=>{current=false};
 },[path,key,enabled,allPages,event.connected,event.revision,refreshKey]);
 return {...state,data:state.key===key?state.data:snapshots.get(key)?.data as T|undefined,fresh:state.key===key&&state.fresh&&event.connected,refresh:()=>setRefreshKey(x=>x+1)};
}
export function useCollection<T>(path:string,enabled=true){return useResource<T[]>(path,enabled,true)}
