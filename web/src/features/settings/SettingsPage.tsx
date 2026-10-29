import {useState} from 'react';
import {useCollection,useResource} from '../../api/resource';
import {command,type Schema} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {Field,Select,Feedback,useAction,when,type Project,type Participant} from '../shared';
import {OutboundSettings} from './OutboundSettings';
export function SettingsPage(){
 const projects=useCollection<Project>('/api/v1/queries/projects');
 const [selection,setSelection]=useState(''),[repos,setRepos]=useState(1);
 const projectId=selection||projects.data?.[0]?.id||'',event=useWorkspaceEvents(),action=useAction();
 const participants=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(projectId),!!projectId);
 const sessions=useCollection<Schema['SessionView']>('/api/v1/queries/sessions?projectId='+encodeURIComponent(projectId),!!projectId);
 const metrics=useResource<{storage:{databaseBytes:number;hardBytes:number;warning:boolean};counters:{commands:number;rejected:number;commandP95Millis:number}}>('/api/v1/queries/metrics');
 const [path,setPath]=useState('');
 return <>
 <div className="actions"><a href="/setup/new" data-local>接入專案</a></div>
 <Select label="管理專案" value={projectId} onChange={e=>setSelection(e.target.value)}><option value="">請選擇專案</option>{projects.data?.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</Select>
 <OutboundSettings projectId={projectId}/>
 <details className="surface"><summary>進階專案登記</summary>
  <section className="surface"><h2>登記專案</h2><p className="muted">只保存名稱及 repository 引用；不讀取專案檔案或更改宿主設定。</p>
   <form action="/api/v1/commands/project.create" method="post" onSubmit={e=>{
    e.preventDefault();const formElement=e.currentTarget,form=new FormData(formElement);
    void action.run(async()=>{
     const names=form.getAll('repoName').map(String),refs=form.getAll('repoRef').map(String);
     const p=await command<Project>('project.create',{name:String(form.get('name')),repositories:names.map((name,i)=>({name,reference:refs[i]})).filter(r=>r.name||r.reference)});
     setSelection(p.result.id);formElement.reset();projects.refresh();action.setNotice('專案已登記。');
    });
   }}><fieldset disabled={!event.connected||action.busy}><legend className="sr-only">專案及 repository 引用</legend><Field label="專案名稱" name="name" required maxLength={256}/>{Array.from({length:repos},(_,i)=><div className="form-row" key={i}><Field label={'Repository '+(i+1)+' 名稱'} name="repoName" maxLength={256}/><Field label={'Repository '+(i+1)+' 引用'} name="repoRef" identifier maxLength={2048}/></div>)}<div className="actions"><button type="submit">登記專案</button><button type="button" className="secondary" disabled={repos>=20} onClick={()=>setRepos(x=>x+1)}>增加 repository</button></div></fieldset></form>
  </section>
 </details><details className="surface"><summary>接入與成員管理</summary><section><h2>專案接入與成員</h2>
   <button disabled={!event.connected||!projectId||action.busy} onClick={()=>void action.run(async()=>{const r=await command<{enrollmentPath:string}>('enrollment.enable',{},projectId);setPath(r.result.enrollmentPath);action.setNotice('已允許此專案的本機 agent 登記。')})}>開放本機 agent 登記</button>
   <ul className="record-list">{participants.data?.map(p=><li key={p.id}><div><strong>{p.name}</strong><p className="mono">{p.id}</p></div><div className="actions"><button className="secondary" disabled={!event.connected||action.busy} onClick={()=>void action.run(async()=>{const r=await command<{credentialPath:string}>('participant.recover',{},p.id);setPath(r.result.credentialPath);action.setNotice('已核發私有恢復檔；仍須明確 resume／handoff。')})}>核發恢復檔</button><button className="danger" disabled={!event.connected||action.busy} onClick={()=>{if(window.confirm('撤銷此 participant 的憑證與專案接入？既有責任需另外移交。'))void action.run(async()=>{await command('participant.revoke',{},p.id);participants.refresh();action.setNotice('已撤銷接入。')})}}>撤銷接入</button></div></li>)}</ul>
   <details><summary>工作階段及最近觀測</summary><ul className="plain-list">{sessions.data?.map(s=><li key={s.id}><span className="mono">{s.id}</span> · {s.presence} · {when(s.lastSeenAt)}<br/><span className="muted">participant {s.participantId} · {s.status} · pending waits {s.pendingWaits}</span></li>)}</ul></details>
   <Feedback error={participants.error??sessions.error}/>
  </section>
 </details><details className="surface"><summary>備份與疑難處理</summary><section><h2>備份、匯出與容量</h2><p className="muted">快照包含已提交的 WAL；可讀匯出排除憑證。兩者只寫入本產品私有目錄。</p><div className="actions"><button disabled={!event.connected||action.busy} onClick={()=>void action.run(async()=>{const r=await command<{path:string}>('backup.create',{});setPath(r.result.path);action.setNotice('一致快照已完成。')})}>建立一致快照</button><button className="secondary" disabled={!event.connected||action.busy} onClick={()=>void action.run(async()=>{const r=await command<{path:string}>('backup.export',{});setPath(r.result.path);action.setNotice('可讀資料已匯出。')})}>匯出可讀資料</button></div>
   {metrics.data?<p className={metrics.data.storage.warning?'notice':'muted'}>SQLite {(metrics.data.storage.databaseBytes/1048576).toFixed(1)} / {(metrics.data.storage.hardBytes/1048576).toFixed(0)} MiB · 命令 {metrics.data.counters.commands} · 拒絕 {metrics.data.counters.rejected} · p95 {metrics.data.counters.commandP95Millis.toFixed(1)} ms</p>:null}
   <p className="muted">WAL、備份、匯出仍會額外使用磁碟。還原需先停止服務，再由你執行本產品的 admin restore；重新啟動後須重新啟用及確認綁定。</p>
  </section>
 </details>
  <Feedback error={action.error??projects.error??metrics.error} notice={action.notice}/>
  {path?<p className="surface mono">{path}<br/>恢復及接入檔案屬私有資料，請只引用路徑。</p>:null}
 </>;
}
