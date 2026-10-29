import {createContext,useContext,useEffect,useState,useSyncExternalStore,type MouseEvent} from 'react';
import {initializeOwner,type OwnerContext,ApiError} from './api/client';
import {useWorkspaceEvents,workspaceEvents} from './api/events';
import {clearSnapshots} from './api/resource';
import {I18nProvider} from './i18n/I18nProvider';
import {NotActivated} from './shell/NotActivated';
import {parseRoute} from './shell/routes';
import {RoutePage} from './shell/RoutePage';
import {Shell} from './shell/Shell';

const Owner=createContext<OwnerContext|undefined>(undefined);
export const useOwner=()=>useContext(Owner);
const pathSnapshot=()=>window.location.pathname;
function subscribePath(fn:()=>void){window.addEventListener('popstate',fn);return()=>window.removeEventListener('popstate',fn)}
// Redirects replace the entry so Back never returns to a path that only forwards.
export function navigate(path:string,replace=false){
 if(replace)history.replaceState(null,'',path);else history.pushState(null,'',path);
 window.dispatchEvent(new PopStateEvent('popstate'));
}
function follow(e:MouseEvent<HTMLElement>){
 const anchor=(e.target as HTMLElement).closest<HTMLAnchorElement>('a[data-local]');
 if(!anchor||e.button!==0||e.ctrlKey||e.metaKey||e.shiftKey||e.altKey)return;
 e.preventDefault();navigate(anchor.pathname);
}
export function App(){
 const path=useSyncExternalStore(subscribePath,pathSnapshot);
 const [owner,setOwner]=useState<OwnerContext>();
 const [loading,setLoading]=useState(true);
 const [problem,setProblem]=useState('');
 const events=useWorkspaceEvents();
 useEffect(()=>{
  let current=true;
  initializeOwner().then(x=>{if(current)setOwner(x)}).catch((error:unknown)=>{if(current)setProblem(error instanceof ApiError?error.code:'offline')}).finally(()=>{if(current)setLoading(false)});
  return()=>{current=false};
 },[]);
 useEffect(()=>{if(owner)void workspaceEvents.start();return()=>workspaceEvents.stop()},[owner]);
 useEffect(()=>{if(events.authorizationRequired){setOwner(undefined);setProblem('unauthorized');clearSnapshots()}},[events.authorizationRequired]);
 const retry=async()=>{
  setLoading(true);
  try{const next=await initializeOwner();setOwner(next);clearSnapshots();await workspaceEvents.reset();setProblem('')}
  catch(error){setProblem(error instanceof ApiError?error.code:'offline')}
  finally{setLoading(false)}
 };
 const route=parseRoute(path);
 return <Owner.Provider value={owner}><I18nProvider ownerLanguage={owner?.ownerLanguage}><div onClick={follow}>
  {owner?<Shell route={route} retry={()=>void retry()} busy={loading}><RoutePage route={route}/></Shell>
   :<div className="rm"><NotActivated checking={loading} offline={problem==='offline'} retry={()=>void retry()}/></div>}
 </div></I18nProvider></Owner.Provider>;
}
