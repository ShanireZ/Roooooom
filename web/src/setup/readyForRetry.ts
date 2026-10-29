import {ApiError,initializeOwner} from '../api/client';
import {workspaceEvents} from '../api/events';

// Re-establishes the owner session and a synchronized stream before an explicit retry.
export async function readyForRetry(){
 await initializeOwner();await workspaceEvents.reset();
 if(workspaceEvents.getSnapshot().connected)return;
 await new Promise<void>((resolve,reject)=>{
  const stop=workspaceEvents.subscribe(()=>{const state=workspaceEvents.getSnapshot();if(state.connected){clearTimeout(timer);stop();resolve()}else if(state.authorizationRequired||state.resetRequired){clearTimeout(timer);stop();reject(new ApiError('unauthorized',401))}});
  const timer=setTimeout(()=>{stop();reject(new ApiError('offline',503))},8000);
 });
}
