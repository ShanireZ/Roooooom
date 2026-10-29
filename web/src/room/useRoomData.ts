import {useEffect,useMemo} from 'react';
import type {Schema} from '../api/client';
import {useCollection} from '../api/resource';
import {useNow} from '../ui/RelativeTime';
import {buildFloor} from './floorModel';

const q=(kind:string,projectId:string)=>'/api/v1/queries/'+kind+'?projectId='+encodeURIComponent(projectId);

// Everything a room renders comes from one request per collection, never per table.
export function useRoomData(projectId:string){
 const tasks=useCollection<Schema['Task']>(q('tasks',projectId));
 const requests=useCollection<Schema['WorkRequest']>(q('requests',projectId));
 const assignments=useCollection<Schema['Assignment']>(q('assignments',projectId));
 const participants=useCollection<Schema['ParticipantView']>(q('participants',projectId));
 const sessions=useCollection<Schema['SessionView']>(q('sessions',projectId));
 const needs=useCollection<Schema['NeedsYouItem']>('/api/v1/queries/needs-you');
 // A synchronization alone emits no event, so "Last synced" needs a light periodic read.
 const refreshSessions=sessions.refresh;
 useEffect(()=>{const timer=setInterval(refreshSessions,60000);return()=>clearInterval(timer)},[refreshSessions]);
 const now=useNow(30000);
 const floor=useMemo(()=>buildFloor({tasks:tasks.data??[],requests:requests.data??[],assignments:assignments.data??[],participants:participants.data??[],sessions:sessions.data??[],
  needsYou:(needs.data??[]).filter(i=>i.projectId===projectId),now}),[tasks.data,requests.data,assignments.data,participants.data,sessions.data,needs.data,projectId,now]);
 return {floor,tasks:tasks.data??[],participants:participants.data??[],
  loaded:tasks.data!==undefined&&requests.data!==undefined&&participants.data!==undefined,
  error:tasks.error??requests.error??assignments.error??participants.error??sessions.error};
}
