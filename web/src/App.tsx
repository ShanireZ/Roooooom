import {createContext,useContext,useEffect,useState,useSyncExternalStore,type MouseEvent} from 'react';
import {initializeOwner,type OwnerContext,ApiError} from './api/client';
import {useWorkspaceEvents,workspaceEvents} from './api/events';
import {clearSnapshots} from './api/resource';
import {ProjectsPage} from './features/projects/ProjectsPage';
import {TaskPage} from './features/tasks/TaskPage';
import {DecisionPage} from './features/decisions/DecisionPage';
import {SettingsPage} from './features/settings/SettingsPage';
import {ProjectSetup} from './features/projects/ProjectSetup';
import {RequestPage} from './features/requests/RequestPage';
import {I18nProvider} from './i18n/I18nProvider';
import {NotActivated} from './shell/NotActivated';
import {parseRoute} from './shell/routes';
import {RoutePage} from './shell/RoutePage';
import {Shell} from './shell/Shell';

const Owner=createContext<OwnerContext|undefined>(undefined);
export const useOwner=()=>useContext(Owner);
const links=[['/projects','專案'],['/decisions','待我處理']];
const pathSnapshot=()=>window.location.pathname;
function subscribePath(fn:()=>void){window.addEventListener('popstate',fn);return()=>window.removeEventListener('popstate',fn)}
export function navigate(path:string){history.pushState(null,'',path);window.dispatchEvent(new PopStateEvent('popstate'))}
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
 if(path==='/rooms'||path.startsWith('/rooms/'))return <Owner.Provider value={owner}><I18nProvider ownerLanguage={owner?.ownerLanguage}><div onClick={follow}>
  {owner?<Shell route={parseRoute(path)} retry={()=>void retry()} busy={loading}><RoutePage route={parseRoute(path)}/></Shell>
   :<div className="rm"><NotActivated checking={loading} offline={problem==='offline'} retry={()=>void retry()}/></div>}
 </div></I18nProvider></Owner.Provider>;
 let title='專案';
 if(path==='/setup'||path.startsWith('/setup/'))title='本機設定';
 else if(path==='/decisions')title='待我處理';
 else if(path==='/settings')title='設定與恢復';
 else if(path.startsWith('/tasks/'))title='任務詳情';
 else if(path.startsWith('/requests/'))title='需求詳情';
 return <Owner.Provider value={owner}><div className="app" onClick={follow}>
  <aside className="sidebar">
   <a className="brand" href="/projects" data-local><span className="brand-mark" aria-hidden="true">R</span>Roooooom</a>
   <p className="sidebar-caption">本機協作工作區</p>
   <nav aria-label="主要導覽">{links.map(([href,label])=><a key={href} href={href} data-local aria-current={path===href?'page':undefined}>{label}</a>)}</nav>
   <p className="sidebar-note">各 agent 在原有環境開工，<br/>在這裡共用進度與決策。</p>
  </aside>
  <div className="workspace">
   <header className="topbar"><span>工作區 / {title}</span><div className="actions"><div className={'connection '+events.connection} role="status">{owner?(events.connected?'已連線':events.connection==='reconciling'?'重新同步中':'已離線 · 僅供檢視'):'尚未連線'}</div><a href="/settings" data-local>設定與恢復</a></div></header>
   <main id="main"><div className="page-heading"><h1>{title}</h1>{owner?<span className="muted">擁有者已啟用</span>:null}</div>
    {owner&&!events.connected?<div className="notice" role="status"><strong>目前僅供檢視</strong><span>{events.lastSyncedAt?'最後同步：'+new Date(events.lastSyncedAt).toLocaleString('zh-TW'):'正在讀取最新狀態。'}</span><button onClick={()=>void retry()} disabled={loading}>重新連線</button></div>:null}
    {events.resetRequired?<div className="notice" role="alert">資料世代或游標已改變，請重新連線以讀取最新版本。</div>:null}
    {!owner?<section className="surface setup"><h2>{loading?'正在確認本機工作階段':'尚未啟用擁有者工作階段'}</h2><p>請從 Roooooom 托盤選擇「重新啟用」，在開啟的頁面點「開始使用」。沒有托盤時，請使用本產品啟動器的重新啟用入口。</p><p className="muted">啟用頁五分鐘內有效。確認後會直接進入本機協作介面。</p>{problem==='offline'?<p role="alert">服務暫時無法連線，請確認 Roooooom 啟動器仍在執行。</p>:null}<button onClick={()=>void retry()} disabled={loading}>重新確認</button></section>:
     path==='/setup'||path.startsWith('/setup/')?<ProjectSetup resumeId={path.startsWith('/setup/')?path.slice(7):''}/>:
     path.startsWith('/tasks/')?<TaskPage key={path} id={path.slice(7)}/>:
     path.startsWith('/requests/')?<RequestPage key={path} id={path.slice(10)}/>:
     path==='/decisions'?<DecisionPage/>:
     path==='/settings'?<SettingsPage/>:
     <ProjectsPage/>}
   </main>
  </div>
 </div></Owner.Provider>;
}
