import {useSyncExternalStore} from 'react';
import {apiRequest,ApiError,permitWrites,subscribeFailures,type Schema} from './client';
type SyncPage=Schema['SyncPage'];
export type EventState={
 connection:'online'|'offline'|'reconciling';connected:boolean;
 lastSyncedAt:string;resetRequired:boolean;revision:number;authorizationRequired:boolean;
};
export class WorkspaceEvents{
 private state:EventState={connection:'offline',connected:false,lastSyncedAt:'',resetRequired:false,revision:0,authorizationRequired:false};
 private listeners=new Set<()=>void>();
 private source?:EventSource;
 private cursor='';
 private serial=0;
 private probing=false;
 private unsubscribeFailure?:()=>void;
 private recovery?:ReturnType<typeof setTimeout>;
 private notify(patch:Partial<EventState>){this.state={...this.state,...patch};for(const listener of this.listeners)listener()}
 getSnapshot=()=>this.state;
 subscribe=(listener:()=>void)=>{this.listeners.add(listener);return()=>{this.listeners.delete(listener)}};
 receive(page:SyncPage){
  clearTimeout(this.recovery);this.recovery=undefined;
  this.cursor=page.cursor;
  this.notify({connection:page.hasMore?'reconciling':'online',connected:!page.hasMore,lastSyncedAt:page.serverTime,resetRequired:false,authorizationRequired:false,revision:this.state.revision+(page.events.length>0?1:0)});
  permitWrites(!page.hasMore);
 }
 disconnect(){permitWrites(false);this.notify({connection:'offline',connected:false});this.scheduleRecovery()}
 private scheduleRecovery(){if(this.recovery||!this.unsubscribeFailure||this.state.authorizationRequired||this.state.resetRequired)return;this.recovery=setTimeout(()=>{this.recovery=undefined;if(this.state.connected||this.state.authorizationRequired||this.state.resetRequired)return;this.source?.close();this.source=undefined;void this.start()},2000)}
 invalidateCursor(){this.source?.close();this.source=undefined;this.cursor='';permitWrites(false);this.notify({resetRequired:true,connected:false,connection:'reconciling'})}
 async start(){
  if(this.source)return;
  if(!this.unsubscribeFailure)this.unsubscribeFailure=subscribeFailures(error=>{this.disconnect();if(error.status===401)this.notify({authorizationRequired:true})});
  const serial=++this.serial;this.notify({connection:'reconciling',connected:false});permitWrites(false);
  try{
   let page:SyncPage;
   do{
    page=(await apiRequest<SyncPage>('/api/v1/sync?cursor='+encodeURIComponent(this.cursor))).result;
    if(serial!==this.serial)return;
    this.receive(page);
   }while(page.hasMore);
   if(serial!==this.serial)return;
   permitWrites(false);this.notify({connected:false,connection:'reconciling'});
   const source=new EventSource('/api/v1/events?cursor='+encodeURIComponent(this.cursor));
   this.source=source;
   source.addEventListener('workspace',event=>{
    if(this.source!==source)return;
    try{this.receive(JSON.parse((event as MessageEvent<string>).data) as SyncPage)}catch{this.disconnect();source.close();this.source=undefined}
   });
   source.onerror=()=>{if(this.source===source){this.disconnect();void this.probe(source)}};
  }catch(error){
   if(serial!==this.serial)return;
   if(error instanceof ApiError && error.code==='cursor_reset')this.invalidateCursor();
   else if(error instanceof ApiError && error.status===401){this.notify({authorizationRequired:true});this.disconnect()}
   else this.disconnect();
  }
 }
 private async probe(source:EventSource){
  if(this.probing)return;this.probing=true;
  try{
   const page=await apiRequest<SyncPage>('/api/v1/sync?cursor='+encodeURIComponent(this.cursor));
   if(this.source===source){
    this.cursor=page.result.cursor;
    this.notify({lastSyncedAt:page.result.serverTime,revision:this.state.revision+(page.result.events.length>0?1:0)});
   }
  }catch(error){
   if(this.source!==source)return;
   if(error instanceof ApiError && error.code==='cursor_reset')this.invalidateCursor();
   if(error instanceof ApiError && error.status===401){source.close();this.source=undefined;this.notify({authorizationRequired:true})}
  }finally{this.probing=false}
 }
 stop(){this.serial++;this.source?.close();this.source=undefined;this.unsubscribeFailure?.();this.unsubscribeFailure=undefined;clearTimeout(this.recovery);this.recovery=undefined;this.disconnect()}
 async reset(){this.stop();this.cursor='';this.notify({resetRequired:false});await this.start()}
}
export const workspaceEvents=new WorkspaceEvents();
export function useWorkspaceEvents(){return useSyncExternalStore(workspaceEvents.subscribe,workspaceEvents.getSnapshot)}
