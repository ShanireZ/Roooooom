import {useState} from 'react';
import {command} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {readyForRetry} from '../projects/ProjectSetup';
import {Field,Feedback,useAction,Toggle} from '../shared';
type Inspection={repository:string;repositoryId:number;private:boolean;hasIssues:boolean;issuesWriteVerified:false};
const defaultEvents=['task.create','integration.record','task.complete','task.cancel','task.reopen'];
export function GuidedOutbound({projectId,onEnabled}:{projectId:string;onEnabled:()=>void}){
 const [kind,setKind]=useState(''),[reference,setReference]=useState(''),[secret,setSecret]=useState(''),[token,setToken]=useState(''),[ref,setRef]=useState(''),[inspection,setInspection]=useState<Inspection>(),[confirmed,setConfirmed]=useState(false),[done,setDone]=useState(false),[custom,setCustom]=useState<string[]>(defaultEvents);
 const action=useAction(),event=useWorkspaceEvents();
 const select=(value:string)=>{setKind(value);setReference('');setSecret('');setToken('');setRef('');setInspection(undefined);setConfirmed(false);setDone(false);setCustom(defaultEvents)};
 const prepare=()=>void action.run(async()=>{
  if(!event.connected)await readyForRetry();
  let credential=ref;
  if(!credential){const r=await command<{credentialRef:string}>('binding.credential',{kind,...(kind==='github'?{token}:{webhookUrl:reference,secret})});credential=r.result.credentialRef;setRef(credential);setToken('');setSecret('')}
  if(kind==='github'){const result=await command<Inspection>('binding.inspect',{repository:reference,credentialRef:credential});setInspection(result.result)}
  setConfirmed(true);
 });
 const enable=()=>void action.run(async()=>{
  if(!event.connected)await readyForRetry();
  await command('binding.create',{projectId,kind,credentialRef:ref,enabled:true,...(kind==='github'?{repository:inspection!.repository,repositoryId:inspection!.repositoryId,private:inspection!.private,fields:['taskId','goal','state','scope','acceptance'],events:custom}:{fields:['projectTitle','goal','summary'],events:['decision-pending','human-acceptance','owner-blocker']})});
  setDone(true);setReference('');onEnabled();action.setNotice(kind==='github'?'GitHub 紀錄已啟用。':'釘釘通知已啟用。');
 });
 return <div className="guided-outbound"><div className="actions"><button className="secondary" disabled={action.busy||!projectId} onClick={()=>select('github')}>連接 GitHub</button><button className="secondary" disabled={action.busy||!projectId} onClick={()=>select('dingtalk')}>連接釘釘</button></div>
 {kind&&!done?<div className="surface"><h3>{kind==='github'?'連接 GitHub 紀錄':'連接釘釘通知'}</h3>{!confirmed?<form onSubmit={e=>{e.preventDefault();prepare()}}><fieldset disabled={action.busy}>
  <Field label={kind==='github'?'Repository 網址':'完整 Webhook 網址'} value={reference} onChange={e=>setReference(e.target.value)} readOnly={!!ref} required maxLength={2048} autoComplete="off"/>
  {!ref?kind==='github'?<Field label="細粒度 PAT" value={token} onChange={e=>setToken(e.target.value)} type="password" autoComplete="new-password" required maxLength={512}/>:<Field label="加簽 secret" value={secret} onChange={e=>setSecret(e.target.value)} type="password" autoComplete="new-password" required maxLength={512}/>:<p className="muted">憑證已保存，可繼續此步驟。若要更換輸入，重新點選連接入口。</p>}
  <p className="muted">{kind==='github'?'只讀取官方 repository 中繼資料。請限定此 repository 與 Issues 權限；不讀取其他工具的登入。':'貼上官方完整網址即可。儲存及啟用不會發送測試訊息。'}</p>
  <button type="submit">{kind==='github'?'確認目的地':'確認通知範圍'}</button>
 </fieldset></form>:<><p>{kind==='github'?`目的地：${inspection!.repository} · ${inspection!.private?'Private':'Public'}`:'目的地：你提供的釘釘群機器人'}</p>{kind==='github'?<><p>一任務一 Issue，追加整合里程碑及收尾紀錄。</p><p className="notice">尚未驗證 Issues 寫入權限。</p><details><summary>自訂內容</summary>{defaultEvents.map(value=><Toggle key={value} label={value} checked={custom.includes(value)} onChange={e=>setCustom(e.target.checked?[...custom,value]:custom.filter(x=>x!==value))}/>)}</details></>:<p>只通知待拍板、人工驗收與需你處理的阻塞。</p>}<p className="muted">啟用後按上述範圍投遞；正式回覆仍在本機決策頁。</p><button disabled={action.busy} onClick={enable}>{kind==='github'?'啟用 GitHub 紀錄':'啟用釘釘通知'}</button></> }</div>:null}
 <Feedback error={action.error} notice={action.notice}/></div>;
}
