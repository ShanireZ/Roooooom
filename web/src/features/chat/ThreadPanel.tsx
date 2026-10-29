import {useId,useState} from 'react';
import {command,type Schema} from '../../api/client';
import {useCollection,useResource} from '../../api/resource';
import {useWorkspaceEvents} from '../../api/events';
import {Area,Select,Toggle,Feedback,useAction,when,memberName,type Task,type Participant} from '../shared';
type Message=Schema['Message']&{receipt?:{state:string;version:number};messageRef?:string};
export function ThreadPanel({projectId,task}:{projectId:string;task?:Task}){
 const path='/api/v1/queries/thread?'+(task?'targetId='+encodeURIComponent(task.id):'projectId='+encodeURIComponent(projectId));
 const messages=useCollection<Message>(path),event=useWorkspaceEvents(),action=useAction();
 const roster=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(projectId));
 const names=Object.fromEntries(roster.data?.map(p=>[p.id,memberName(p,roster.data??[])])??[]);
 const [body,setBody]=useState('');
 return <section className="surface"><div className="section-heading"><h2>{task?'任務討論':'專案聊天室'}</h2><span className="muted">聊天不構成正式拍板</span></div>
  <Feedback error={messages.error}/>
  <div className="messages">{messages.data?.map(message=><MessageRow key={message.id} message={message} names={names}/>)}</div>
  {!messages.data?.length?<p className="muted">尚無討論。傳送後的收件人由服務保存，不因之後換整合者而變動。</p>:null}
  <form action="/api/v1/commands/message.send" method="post" onSubmit={e=>{
   e.preventDefault();const form=new FormData(e.currentTarget);
   void action.run(async()=>{
    await command('message.send',{projectId,taskId:task?.id,threadId:task?'task':'project',purpose:String(form.get('purpose')??'discussion'),priority:'normal',body,recipientMode:form.get('actionRequired')==='on'&&task?.integratorId?'integrator':String(form.get('recipient')??'project'),recipientIds:[],actionRequired:form.get('actionRequired')==='on'});
    setBody('');messages.refresh();
   });
  }}><fieldset disabled={!event.connected||action.busy}><legend className="sr-only">傳送討論訊息</legend>
   <Area label="訊息內容" name="body" value={body} onChange={e=>setBody(e.target.value)} required maxLength={16384}/>
   <Toggle label={task?.integratorId?'請負責 agent 處理':'請專案 agent 處理'} name="actionRequired"/>
   <details><summary>收件範圍與用途</summary><div className="form-row"><Select label="訊息用途" name="purpose" defaultValue="discussion"><option value="discussion">討論</option><option value="question">問題</option><option value="proposal">提案</option><option value="status">進度</option><option value="contract-change">介面變更</option></Select>
    <Select label="收件範圍" name="recipient" defaultValue="project"><option value="project">專案成員</option>{task?.integratorId?<option value="integrator">目前整合者</option>:null}</Select></div>
   </details>
   <button type="submit">傳送訊息</button>
  </fieldset><Feedback error={action.error}/></form>
 </section>;
}
function MessageRow({message,names}:{message:Message;names:Record<string,string>}){
 const [expanded,setExpanded]=useState(false);
 const receiptsId=useId();
 const detail=useResource<Schema['Message']&{receipts?:Array<{participantId:string;state:string;version:number}>}>('/api/v1/queries/message?targetId='+encodeURIComponent(message.id),expanded||!!message.messageRef);
 const data=detail.data??message;
 return <article className="message"><div className="message-meta"><strong data-testid="message-actor">{data.actorKind==='human'?'你':data.actorKind+' · '+(names[data.actorId]??'agent')}</strong><time>{data.receivedAt?when(data.receivedAt):''}</time></div>
  {data.imported?<p className="attention">離線補登 · 原始發生：{data.localOccurredAt?when(data.localOccurredAt):'見來源紀錄'} · 服務收到：{when(data.receivedAt)}</p>:null}
  <pre>{data.body??'讀取完整訊息中…'}</pre><p className="muted">{data.purpose} · {data.priority} · {data.actionRequired?'需要處理':'討論訊息'}</p>
  <button className="text-button" type="button" aria-expanded={expanded} aria-controls={receiptsId} onClick={()=>setExpanded(x=>!x)}>檢視送達狀態</button>
  {expanded?<div id={receiptsId}><ul className="plain-list">{detail.data?.receipts?.map(r=><li key={r.participantId}><span>{names[r.participantId]??r.participantId}</span> · {r.state} · v{r.version}</li>)}</ul><Feedback error={detail.error}/></div>:null}
 </article>;
}
