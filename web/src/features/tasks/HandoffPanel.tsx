import {useRef,useState} from 'react';
import {command,type Schema} from '../../api/client';
import {useCollection} from '../../api/resource';
import {Field,Select,Toggle,Feedback,useAction,versions,when,memberName,type Task,type Assignment,type Participant} from '../shared';
export function HandoffPanel({task,assignments,fresh}:{task:Task;assignments:Assignment[];fresh:boolean}){
 const [open,setOpen]=useState(false),[notice,setNotice]=useState('');
 return <><details open={open} onToggle={e=>setOpen(e.currentTarget.open)}><summary>強制交接整合責任</summary>{open?<HandoffForm task={task} assignments={assignments} fresh={fresh} done={()=>{setOpen(false);setNotice('整合責任已明確交接。')}}/>:null}</details><Feedback notice={notice}/></>;
}
function HandoffForm({task,assignments,fresh,done}:{task:Task;assignments:Assignment[];fresh:boolean;done:()=>void}){
 const sessions=useCollection<Schema['SessionView']>('/api/v1/queries/sessions?projectId='+encodeURIComponent(task.projectId));
 const roster=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(task.projectId));
 const base=useRef(versions(task)).current,action=useAction(),[target,setTarget]=useState('');
 const stale=task.version!==base.expectedVersion;
 const claims=assignments.filter(a=>a.claimParticipantId===task.integratorId&&!['accepted','cancelled'].includes(a.state));
 return <form className="inset" action="/api/v1/commands/integrator.handoff" method="post" onSubmit={e=>{
  e.preventDefault();const form=new FormData(e.currentTarget);
  void action.run(async()=>{
   const dispositions=claims.map(a=>{const disposition=String(form.get('disposition-'+a.id));return {assignmentId:a.id,action:disposition,targetSessionId:disposition==='release'?undefined:disposition==='continue-worker'?task.integratorSessionId:target}});
   await command('integrator.handoff',{fromSessionId:task.integratorSessionId,toSessionId:target,dispositions,reason:String(form.get('reason')),force:true},task.id,base);
   done();
  });
 }}><p>這會更換整合責任並重新計算協作者名額。請逐項處置舊整合者的未完成分工；各家宿主程序仍由你管理。</p>
  <fieldset disabled={!fresh||!sessions.fresh||stale||action.busy}><legend>由你明確指定本次交接</legend>
   <Select label="新的整合工作階段" name="target" value={target} onChange={e=>setTarget(e.target.value)} required><option value="">請選擇</option>{sessions.data?.filter(s=>s.status==='active'&&s.participantId!==task.integratorId).map(s=><option key={s.id} value={s.id}>{roster.data?.find(p=>p.id===s.participantId)?memberName(roster.data.find(p=>p.id===s.participantId)!,roster.data):'agent'} · 最近同步 {when(s.lastSeenAt)} · {s.id.slice(0,8)}</option>)}</Select>
   {claims.map(a=><Select key={a.id} label={'分工 '+a.role+' 的處置'} name={'disposition-'+a.id} defaultValue="" required><option value="">逐項選擇</option><option value="transfer">移交給新的整合者</option><option value="release">釋放，留待重新認領</option><option value="continue-worker">舊整合者轉為協作者，依 N 名額檢查</option></Select>)}
   <Field label="強制交接理由" name="reason" required maxLength={8192}/>
   <Toggle label="確認逐項處置並更換整合責任" name="confirm" required/>
   <button type="submit">確認強制交接</button>
  </fieldset>{stale?<p className="notice">任務版本已變更，請關閉並重新開啟交接表單，重看每份責任。</p>:null}<Feedback error={action.error??sessions.error}/>
 </form>;
}
