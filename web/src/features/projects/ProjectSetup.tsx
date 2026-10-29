import {useState} from 'react';
import {command,initializeOwner,ApiError} from '../../api/client';
import {useCollection,useResource} from '../../api/resource';
import {workspaceEvents,useWorkspaceEvents} from '../../api/events';
import {navigate} from '../../App';
import {Field,Feedback,useAction,type Project} from '../shared';

type SetupProject=Project&{enrollmentEnabled?:boolean};
type ConnectionInfo={clientPath:string;skillPath:string;dataDir:string;serviceURL:string};
const suggestedName=(reference:string)=>reference.trim().replace(/[\\/]+$/,'').split(/[\\/]/).pop()?.replace(/\.git$/,'').slice(0,256)??'';
export async function readyForRetry(){
 await initializeOwner();await workspaceEvents.reset();
 if(workspaceEvents.getSnapshot().connected)return;
 await new Promise<void>((resolve,reject)=>{
  const stop=workspaceEvents.subscribe(()=>{const state=workspaceEvents.getSnapshot();if(state.connected){clearTimeout(timer);stop();resolve()}else if(state.authorizationRequired||state.resetRequired){clearTimeout(timer);stop();reject(new Error('請重新確認本機身分。'))}});
  const timer=setTimeout(()=>{stop();reject(new Error('服務尚未恢復，請稍後再試。'))},8000);
 });
}
export function ProjectSetup({resumeId=''}:{resumeId?:string}){
 const projects=useCollection<SetupProject>('/api/v1/queries/projects'),events=useWorkspaceEvents(),action=useAction();
 const [saved,setSaved]=useState<SetupProject>(),[drafting,setDrafting]=useState(false),[uncertain,setUncertain]=useState(false),[name,setName]=useState(''),[named,setNamed]=useState(false),[reference,setReference]=useState(''),[extra,setExtra]=useState<string[]>([]),[copied,setCopied]=useState(false);
 const target=resumeId==='new'?'':resumeId;
 const project=saved&&(!target||saved.id===target)?saved:projects.data?.find(p=>p.id===target);
 const connected=project?.enrollmentEnabled===true;
 const info=useResource<ConnectionInfo>('/api/v1/setup/connection',connected);
 const open=async(p:SetupProject)=>{await command('enrollment.enable',{},p.id);setSaved({...p,enrollmentEnabled:true});projects.refresh()};
 const text=project&&info.data?[
  '請依 Roooooom 共享 skill 接入這個專案；只操作下列已登記專案。',
  '專案：'+project.name,
  'Repository 引用：'+project.repositories.map(r=>r.reference).join('；'),
  '先讀取共享 skill：'+info.data.skillPath.replaceAll('\\','/'),
  '本機 CLI：'+info.data.clientPath.replaceAll('\\','/'),
  '資料根：'+info.data.dataDir.replaceAll('\\','/'),
  '服務：'+info.data.serviceURL,
  'projectId：'+project.id,
  '先使用 version --json、help --json 唯讀核對；為自己選一個不與其他 agent 共用的 profile，再用 connect --data-dir <上述資料根> --profile <自己的 profile> --project '+project.id+' --json。',
  '先同步需求、任務與未讀訊息；明確認領後再工作。已有 ended 責任須明確 resume，不能自動接管 active／unknown session。',
  '依專案權威處理 AGENTS.md 接入管理區塊；不要改任何宿主設定、hooks、模型 API 或全域 PATH。'
 ].join('\n'):'';
 if(connected&&project)return <section className="surface setup-flow"><p className="eyebrow">接入已開放 · {project.name}</p><h2>把接入說明交給 agent</h2><p className="muted">複製後貼到原本的對話，由 skill 引導報到。你不需要填入 ID 或憑證。</p><pre className="join-instructions" data-testid="join-instructions">{text||'正在準備本機接入說明…'}</pre><div className="actions"><button disabled={!text} onClick={()=>void navigator.clipboard.writeText(text).then(()=>setCopied(true)).catch(()=>action.setNotice('無法複製時，可直接選取上方說明。'))}>{copied?'已複製':'複製接入說明'}</button><a className="button secondary" href="/projects" data-local>進入專案</a></div><p className="muted">等待 agent 加入期間，可以先留下需求。服務不會啟動或喚醒宿主。</p><Feedback error={info.error??action.error} notice={action.notice}/></section>;
 if(project)return <section className="surface setup-flow"><h2>繼續接入 {project.name}</h2><p>專案已登記，尚未開放 agent 接入。</p><p className="muted">確認後，允許這台電腦同一使用者的 agent 加入此專案。</p><button disabled={action.busy} onClick={()=>void action.run(async()=>{if(!events.connected)await readyForRetry();await open(project)})}>{action.busy?'接入中…':'開放此專案接入'}</button><Feedback error={action.error} notice={action.notice}/></section>;
 if(target&&!projects.data)return <section className="surface"><p>正在讀取已登記的專案…</p><Feedback error={projects.error}/></section>;
 if(target)return <section className="surface"><h2>找不到這個接入中的專案</h2><a href="/setup" data-local>查看已登記專案</a></section>;
 if(resumeId!=='new'&&!drafting&&projects.data?.length)return <section className="surface setup-flow"><h2>繼續使用你的專案</h2><ul className="record-list">{projects.data.map(p=><li key={p.id}><div><strong>{p.name}</strong><p className="muted">{p.enrollmentEnabled?'已開放 agent 接入':'尚未開放接入'}</p></div><a href={'/setup/'+p.id} data-local>{p.enrollmentEnabled?'複製接入說明':'繼續接入'}</a></li>)}</ul><a className="button secondary" href="/setup/new" data-local>接入另一個專案</a><a className="button secondary" href="/projects" data-local>進入專案</a></section>;
 return <section className="surface setup-flow"><p className="eyebrow">先接入一個專案</p><h2>告訴我們是哪個專案</h2><p className="muted">只保存引用，不會掃描磁碟或修改 repository。</p><form onSubmit={e=>{
  e.preventDefault();setDrafting(true);
  void action.run(async()=>{
   if(!events.connected)await readyForRetry();
   const refs=[reference,...extra].map(x=>x.trim()).filter(Boolean);
   let response;
   try{response=await command<SetupProject>('project.create',{name:name.trim(),repositories:refs.map((ref,i)=>({name:i===0?name.trim():suggestedName(ref),reference:ref}))})}
   catch(error){if(error instanceof ApiError&&error.status>=500)setUncertain(true);throw error}
   const p=response.result;setSaved(p);navigate('/setup/'+p.id);await open(p);
  });
 }}><fieldset disabled={action.busy||!projects.fresh||uncertain}><Field label="Repository 路徑或網址" name="reference" identifier value={reference} maxLength={2048} required onChange={e=>{setReference(e.target.value);if(!named)setName(suggestedName(e.target.value))}} placeholder="例如 C:/Projects/MyProject 或 repository 網址"/><Field label="專案名稱" name="name" value={name} required maxLength={256} onChange={e=>{setNamed(true);setName(e.target.value)}}/><details><summary>多個 repository（選填）</summary>{extra.map((ref,i)=><Field key={i} label={'額外 repository '+(i+1)} value={ref} identifier maxLength={2048} onChange={e=>setExtra(xs=>xs.map((x,n)=>n===i?e.target.value:x))}/>)}<button type="button" className="secondary" disabled={extra.length>=19} onClick={()=>setExtra(xs=>[...xs,''])}>增加 repository</button></details><p className="notice">確認接入後，允許這台電腦上同一使用者的 agent 加入「{name||'這個專案'}」。</p></fieldset>{uncertain?<p className="notice">建立結果尚未確認。重試會查回同一筆請求，不會另建專案。</p>:null}<button type="submit" disabled={action.busy||(!projects.fresh&&!uncertain)||!name.trim()||!reference.trim()}>{action.busy?'接入中…':action.error?'重試接入':'確認接入'}</button><Feedback error={action.error} notice={action.notice}/></form></section>;
}
