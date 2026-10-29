import {useMemo} from 'react';
import type {Schema} from '../api/client';
import {useCollection} from '../api/resource';
import {routePath} from './routes';

export type NeedsYouItem=Schema['NeedsYouItem'];
export const needsYouKey=(i:NeedsYouItem)=>[i.kind,i.projectId,i.decisionRequestId,i.candidateId,i.taskId,i.requestId,i.code].join('|');

// Each item opens the drawer that resolves it.
export function needsYouPath(i:NeedsYouItem):string{
 const projectId=i.projectId;
 if(i.kind==='decision'&&i.decisionRequestId)return routePath({name:'room',projectId,drawer:{kind:'decision',id:i.decisionRequestId}});
 if(i.kind==='acceptance'&&i.taskId)return routePath({name:'room',projectId,drawer:{kind:'acceptance',taskId:i.taskId}});
 if(i.taskId)return routePath({name:'room',projectId,drawer:{kind:'task',id:i.taskId,tab:'manage'}});
 if(i.requestId)return routePath({name:'room',projectId,drawer:{kind:'request',id:i.requestId}});
 return routePath({name:'room',projectId});
}

// Ordered by room tab order, then oldest first (spec §6.3).
export function useNeedsYou(projectOrder:readonly string[]){
 const query=useCollection<NeedsYouItem>('/api/v1/queries/needs-you');
 return useMemo(()=>{
  const rank=new Map(projectOrder.map((id,i)=>[id,i]));
  const compare=(a:NeedsYouItem,b:NeedsYouItem)=>(rank.get(a.projectId)??Infinity)-(rank.get(b.projectId)??Infinity)||a.since.localeCompare(b.since);
  const items=[...(query.data??[])].sort(compare);
  const byProject:Record<string,NeedsYouItem[]>={};
  for(const item of items)(byProject[item.projectId]??=[]).push(item);
  // The resolved item may already be gone, so "next" is the first item ordered after it.
  const next=(after:NeedsYouItem)=>{
   const at=items.findIndex(i=>needsYouKey(i)===needsYouKey(after));
   return at>=0?items[at+1]:items.find(i=>compare(i,after)>=0);
  };
  return {items,byProject,next,loaded:query.data!==undefined,error:query.error,fresh:query.fresh,refresh:query.refresh};
 },[query.data,query.error,query.fresh,query.refresh,projectOrder]);
}
