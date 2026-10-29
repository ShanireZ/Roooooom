import {useRef,useState} from 'react';
import {command,type Schema} from '../../api/client';
import {Area,Select,Feedback,when,useAction,memberName,type Participant} from '../shared';
import type {WorkRequest} from './RequestPanel';
type Props={request:WorkRequest;members:Participant[];sessions:Schema['SessionView'][];fresh:boolean;refresh:()=>void};
export function RequestManagement(props:Props){
 const [open,setOpen]=useState(false);
 return <details className="inset" open={open} onToggle={e=>setOpen(e.currentTarget.open)}><summary>管理這份需求</summary>{open?<Controls {...props} done={()=>{setOpen(false);props.refresh()}}/>:null}</details>;
}
function Controls({request:r,members,sessions,fresh,done}:Props&{done:()=>void}){
 const base=useRef({expectedVersion:r.version,claimEpoch:r.preparationEpoch,source:r.preparerSessionId??''}).current,action=useAction(),stale=base.expectedVersion!==r.version;
 return <><p>更換整理責任或取消都會留下理由。過時的表單不能更換新責任。</p><form onSubmit={e=>{e.preventDefault();const form=new FormData(e.currentTarget),submitter=(e.nativeEvent as SubmitEvent).submitter as HTMLButtonElement;void action.run(async()=>{await command(submitter.value==='assign'?'request.assign':'request.cancel',{reason:String(form.get('reason')),...(submitter.value==='assign'?{toSessionId:String(form.get('target')),expectedSourceSessionId:base.source}:{})},r.id,{expectedVersion:base.expectedVersion,claimEpoch:base.claimEpoch});done()})}}><fieldset disabled={!fresh||stale||action.busy}><Select label="新的整理 agent" name="target"><option value="">請選擇（取消需求不用選）</option>{sessions.filter(s=>s.status==='active'&&s.id!==r.preparerSessionId).map(s=>{const p=members.find(p=>p.id===s.participantId);return <option key={s.id} value={s.id}>{p?memberName(p,members):'未命名 agent'} · 最近同步 {when(s.lastSeenAt)} · {s.id.slice(0,8)}</option>})}</Select><Area label="處理理由" name="reason" required maxLength={8192}/><div className="actions"><button type="submit" value="assign">確認更換整理者</button><button className="danger" type="submit" value="cancel">確認取消需求</button></div></fieldset></form>{stale?<p className="notice">需求版本已變更，請關閉並重新開啟管理表單，重新確認負責人。</p>:null}<Feedback error={action.error}/></>;
}
