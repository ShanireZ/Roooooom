import {useCollection} from '../../api/resource';
import {command} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {Field,Select,Toggle,Feedback,useAction} from '../shared';
import {GuidedOutbound} from './GuidedOutbound';
type Binding={id:string;projectId:string;version:number;kind:string;repository:string;repositoryId:number;private:boolean;credentialRef:string;fields:string[];events:string[];enabled:boolean;needsRebind:boolean};
type Job={jobId:string;taskId:string;bindingId:string;version:number;kind:string;status:string;errorCode?:string;attempts:number;outcomeUncertain?:boolean;retryAt?:string};
const fields=[['taskId','任務 ID'],['goal','任務目標'],['state','任務階段'],['scope','範圍摘要'],['acceptance','驗收條件']];
const events=[['task.create','建立任務'],['integration.record','整合里程碑'],['task.complete','完成'],['task.cancel','取消'],['task.reopen','重新開啟'],['decision.request','待決事項'],['decision.resolve','正式決策'],['decision.supersede','取代決策'],['delivery.submit','交付'],['delivery.review','交付接收'],['acceptance.candidate','驗收候選'],['acceptance.accept','接受結果']];
export function OutboundSettings({projectId}:{projectId:string}){
 const bindings=useCollection<Binding>('/api/v1/queries/bindings?projectId='+encodeURIComponent(projectId),!!projectId);
 const jobs=useCollection<Job>('/api/v1/queries/outbound-jobs?projectId='+encodeURIComponent(projectId),!!projectId);
 const action=useAction(),event=useWorkspaceEvents();
 const [credential,setCredential]=useState({kind:'',reference:''});
 return <section className="surface"><h2>外部整合</h2>
  <p className="muted">GitHub 只記錄你勾選的任務摘要與里程碑。遠端 Issue 的修改不會更改本機任務。啟用後可對指定 repository 寫入紀錄。</p>
  <p className="muted">憑證只存入本產品私有資料根；一般查詢與收據只回傳參考名稱。服務不讀取 gh 或其他宿主的登入資料。儲存及建立綁定不會自動測試發送。</p>
  <GuidedOutbound key={projectId} projectId={projectId} onEnabled={()=>bindings.refresh()}/>
  <ul className="plain-list">{bindings.data?.filter(b=>b.enabled).map(b=><li key={b.id}>{b.kind==='github'?'GitHub · '+b.repository:'釘釘通知'} · {b.needsRebind?'需要重新確認':'已啟用'}</li>)}</ul>
  <details><summary>進階綁定與憑證管理</summary>
  <CredentialForm onSaved={(kind,reference)=>setCredential({kind,reference})}/>
  <form action="/api/v1/commands/binding.create" method="post" onSubmit={e=>{
   e.preventDefault();const element=e.currentTarget,form=new FormData(element);
   void action.run(async()=>{
    await command('binding.create',{projectId,kind:'github',repository:String(form.get('repository')),repositoryId:Number(form.get('repositoryId')),private:form.get('visibility')==='private',credentialRef:String(form.get('credentialRef')),fields:form.getAll('fields').map(String),events:form.getAll('events').map(String),enabled:true});
    element.reset();bindings.refresh();action.setNotice('GitHub 綁定已建立。');
   });
  }}><fieldset disabled={!projectId||!event.connected||action.busy}><legend>新 GitHub 綁定</legend>
   <Field label="GitHub repository（owner/repo）" name="repository" identifier required pattern="[A-Za-z0-9_-]+/[A-Za-z0-9_.-]+" maxLength={256}/>
   <Field label="Canonical repository ID" name="repositoryId" type="number" min={1} step={1} required/>
   <Select label="已確認的 repository 公開性" name="visibility" defaultValue="private"><option value="private">Private</option><option value="public">Public</option></Select>
   <Field key={credential.kind==='github'?credential.reference:'pat'} label="PAT 私有檔參考" name="credentialRef" defaultValue={credential.kind==='github'?credential.reference:''} identifier required pattern="[a-zA-Z0-9_-]{1,128}" maxLength={128}/>
   <fieldset><legend>可投遞欄位</legend>{fields.map(([value,label])=><Toggle key={value} label={label} name="fields" value={value} defaultChecked/>)}</fieldset>
   <fieldset><legend>可投遞事件</legend>{events.map(([value,label])=><Toggle key={value} label={label} name="events" value={value} defaultChecked/>)}</fieldset>
   <button type="submit">建立 GitHub 綁定</button>
  </fieldset></form>
  <form action="/api/v1/commands/binding.create" method="post" onSubmit={e=>{
   e.preventDefault();const form=new FormData(e.currentTarget);
   void action.run(async()=>{await command('binding.create',{projectId,kind:'dingtalk',credentialRef:String(form.get('credentialRef')),enabled:true,fields:form.getAll('fields').map(String),events:form.getAll('events').map(String)});bindings.refresh();action.setNotice('釘釘通知綁定已建立。')});
  }}><fieldset disabled={!projectId||!event.connected||action.busy}><legend>新釘釘通知綁定</legend>
   <Field key={credential.kind==='dingtalk'?credential.reference:'bot'} label="釘釘私有憑證參考" name="credentialRef" defaultValue={credential.kind==='dingtalk'?credential.reference:''} identifier required pattern="[a-zA-Z0-9_-]{1,128}" maxLength={128}/>
   <fieldset><legend>通知內容</legend>{[['taskId','任務 ID'],['goal','任務標題'],['projectTitle','專案標題'],['summary','待辦短摘要']].map(([value,label])=><Toggle key={value} label={label} name="fields" value={value} defaultChecked/>)}</fieldset>
   <fieldset><legend>需我處理的事項</legend>{[['decision-pending','待拍板'],['human-acceptance','指定人工驗收'],['owner-blocker','需我處理的阻塞']].map(([value,label])=><Toggle key={value} label={label} name="events" value={value} defaultChecked/>)}</fieldset>
   <details><summary>通知格式預覽</summary><p>Roooooom · 有待你拍板的事項<br/>Request ID · 版本<br/>專案／任務標題與短摘要（依勾選項目）<br/>NotificationId（未知結果重試時保留）</p><p>手機只能收提醒，請回這台電腦的決策頁正式回覆。</p></details>
   <p className="muted">同一 bot 跨專案最多 10 次／分鐘，不 @全員。暫時錯誤最多自動嘗試 3 次，逾限暫停；網路結果未知可能重複同一 NotificationId。accepted 只代表釘釘平台接受。</p>
   <button type="submit">建立釘釘通知綁定</button>
  </fieldset></form>
  <ul className="record-list">{bindings.data?.map(b=><li key={b.id}><div><strong className="binding-name">{b.kind} · {b.repository||b.credentialRef}</strong><p>{b.needsRebind?'還原後需重新確認':b.enabled?'已啟用':'已停用'} · v{b.version} · {b.kind==='github'?(b.private?'private':'public'):'單向通知'}</p><p className="mono">{b.id}</p><form onSubmit={e=>{e.preventDefault();const form=new FormData(e.currentTarget);void action.run(async()=>{await command('binding.rotate',{credentialRef:String(form.get('credentialRef')),reason:String(form.get('reason'))},b.id,{expectedVersion:b.version});bindings.refresh();action.setNotice('憑證參考已更新；原目標與未知結果保護保留。')})}}><fieldset disabled={!b.enabled||b.needsRebind||!event.connected||action.busy}><Field key={credential.reference} label={'新憑證參考 '+b.id.slice(0,8)} name="credentialRef" defaultValue={credential.kind===b.kind?credential.reference:''} identifier required pattern="[a-zA-Z0-9_-]{1,128}"/><Field label={'更換理由 '+b.id.slice(0,8)} name="reason" required maxLength={8192}/><button type="submit" className="secondary">更新此綁定的憑證</button></fieldset></form></div><button className="secondary" disabled={!event.connected||action.busy||!b.enabled} onClick={()=>void action.run(async()=>{await command('binding.disable',{},b.id,{expectedVersion:b.version});bindings.refresh();action.setNotice('已停用；舊工作不會轉投新綁定。')})}>停用綁定</button></li>)}</ul>
  <details><summary>投遞狀態（GitHub uncertain 需對帳；釘釘 accepted 只代表平台接受）</summary><ul className="plain-list">{jobs.data?.map(j=><li key={j.jobId}><span className="mono">{j.jobId}</span> · {j.kind} · {j.status} · attempts {j.attempts}{j.errorCode?' · '+j.errorCode:''}{(j.status==='uncertain'||j.status==='paused')?<form onSubmit={e=>{e.preventDefault();const reason=String(new FormData(e.currentTarget).get('reason'));void action.run(async()=>{await command('binding.retry',{mode:j.status==='uncertain'||(j.outcomeUncertain&&j.kind!=='dingtalk.notify')?'reconcile':'retry',reason},j.jobId,{expectedVersion:j.version});jobs.refresh();action.setNotice('已登記原綁定的處理要求。')})}}><fieldset disabled={!event.connected||action.busy}><Field label={'處理理由 '+j.jobId.slice(0,8)} name="reason" required maxLength={8192}/><button type="submit">{j.status==='uncertain'?'重新對帳／有限重試':'依原綁定重試'}</button></fieldset></form>:null}</li>)}</ul></details>
  <p className="muted">換 repository 時先停用舊綁定，再明確建立新綁定；舊工作保留原目標。uncertain 不會盲目重建 Issue／comment。恢復資料庫後也不會自動重綁或回放。</p>
  </details><Feedback error={action.error??bindings.error??jobs.error} notice={action.notice}/>
 </section>;
}
import {useState} from 'react';
function CredentialForm({onSaved}:{onSaved:(kind:string,reference:string)=>void}){
 const [kind,setKind]=useState('github'),action=useAction(),event=useWorkspaceEvents();
 return <form action="/api/v1/commands/binding.credential" method="post" onSubmit={e=>{
  e.preventDefault();const element=e.currentTarget,form=new FormData(element);
  void action.run(async()=>{const r=await command<{credentialRef:string}>('binding.credential',{kind,token:String(form.get('token')),...(kind==='dingtalk'?{secret:String(form.get('secret'))}:{})});element.reset();onSaved(kind,r.result.credentialRef);action.setNotice('已存入私有憑證檔；參考名稱可供綁定使用。')});
 }}><fieldset disabled={!event.connected||action.busy}><legend>儲存外部整合憑證</legend><Select label="憑證類型" value={kind} onChange={e=>setKind(e.target.value)}><option value="github">GitHub 細粒度 PAT</option><option value="dingtalk">釘釘 Webhook</option></Select>
  <Field label="Token（只存入本產品私有資料）" name="token" type="password" required maxLength={512} autoComplete="new-password"/>
  {kind==='dingtalk'?<><p className="muted">只填官方 Webhook URL 中 access_token 的值。端點固定為釘釘官方 HTTPS；加簽必填。</p><Field label="加簽 secret" name="secret" type="password" required maxLength={512} autoComplete="new-password"/></>:null}
  <button type="submit">儲存私有憑證</button>
 </fieldset><Feedback error={action.error} notice={action.notice}/></form>;
}
