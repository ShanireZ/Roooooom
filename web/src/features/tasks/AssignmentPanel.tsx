import {useRef,useState} from 'react';
import {useCollection} from '../../api/resource';
import {command} from '../../api/client';
import {Field,Area,Select,Toggle,Feedback,List,lines,useAction,states,versions,memberName,type Task,type Assignment,type Project,type Participant} from '../shared';
import {HandoffPanel} from './HandoffPanel';
export function AssignmentPanel({task,project,fresh}:{task:Task;project:Project;fresh:boolean}){
 const rows=useCollection<Assignment>('/api/v1/queries/assignments?targetId='+encodeURIComponent(task.id));
 const [open,setOpen]=useState(false);
 const roster=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(task.projectId));
 const names=Object.fromEntries(roster.data?.map(p=>[p.id,memberName(p,roster.data??[])])??[]);
 return <section className="surface"><div className="section-heading"><h2>分工與依賴</h2><button className="secondary" disabled={!fresh||!task.integratorId||['done','cancelled'].includes(task.phase)} onClick={()=>setOpen(x=>!x)} aria-expanded={open} aria-controls="new-assignment">新增分工</button></div>
  {!task.integratorId?<p className="muted">整合者接手後即可拆分工作；不必等協作者到齊。</p>:null}
  {open?<AssignmentForm key={task.id} task={task} project={project} assignments={rows.data??[]} fresh={fresh} onDone={()=>{setOpen(false);rows.refresh()}}/>:null}
  <Feedback error={rows.error}/>
  <div className="assignment-list">{rows.data?.map(a=><article key={a.id} className="assignment"><div className="section-heading"><h3>{a.role}</h3><span>{states[a.state]??a.state}</span></div><p>{a.output}</p><p className="muted">輸入：{a.input}</p><p className="muted">負責人：<span className="mono">{a.claimParticipantId?names[a.claimParticipantId]??'負責 agent':'開放認領'}</span> · {a.canStart?'依賴已滿足':'目前不可開工'}</p>{a.cancellationReason?<p>{a.cancellationReason}</p>:null}<details><summary>寫入邊界與依賴</summary><List items={a.writeScope}/><p className="mono">{a.repoId} @ {a.baseRef}</p><List items={a.dependencies}/><p className="muted">contract v{a.contractVersion} · claim epoch {a.claimEpoch}</p></details>{!['done','cancelled'].includes(task.phase)&&!['accepted','cancelled'].includes(a.state)?<CancelAssignment task={task} assignment={a} fresh={fresh&&rows.fresh} onDone={rows.refresh}/>:null}</article>)}</div>
  {!rows.data?.length&&!open?<p className="muted">尚無分工。每份責任只接受一個有效認領。</p>:null}
  {task.integratorId&&!['done','cancelled'].includes(task.phase)?<HandoffPanel task={task} assignments={rows.data??[]} fresh={fresh&&rows.fresh}/>:null}
 </section>;
}
function CancelAssignment({task,assignment,fresh,onDone}:{task:Task;assignment:Assignment;fresh:boolean;onDone:()=>void}){
 const [base,setBase]=useState<{taskVersion:number;meta:ReturnType<typeof versions>&{claimEpoch:number}}|null>(null),action=useAction();
 const stale=base!==null&&(base.taskVersion!==task.version||base.meta.expectedVersion!==assignment.version);
 return <><button className="secondary" disabled={!fresh||action.busy} onClick={()=>setBase(base?null:{taskVersion:task.version,meta:{...versions(task),expectedVersion:assignment.version,claimEpoch:assignment.claimEpoch}})} aria-expanded={base!==null}>{base?'關閉取消表單':'取消此分工'}</button>
  {base?<form className="inset" action="/api/v1/commands/assignment.update" method="post" onSubmit={e=>{e.preventDefault();const form=new FormData(e.currentTarget);void action.run(async()=>{await command('assignment.update',{state:'cancelled',reason:String(form.get('reason'))},assignment.id,base.meta);setBase(null);onDone()})}}>
   <p>取消後不再開放認領，既有交付保留；依賴此分工的工作仍受阻。協作者名額不會自動釋放。</p>
   <fieldset disabled={!fresh||stale||action.busy}><Area label="取消分工的理由" name="reason" required maxLength={8192}/><button type="submit">確認取消分工</button></fieldset>
   {stale?<p className="notice">分工或任務版本已變更，請關閉並重新開啟表單。</p>:null}<Feedback error={action.error}/>
  </form>:null}</>;
}
function AssignmentForm({task,project,assignments,fresh,onDone}:{task:Task;project:Project;assignments:Assignment[];fresh:boolean;onDone:()=>void}){
 const base=useRef(versions(task)).current,action=useAction(),stale=base.expectedVersion!==task.version;
 return <form id="new-assignment" className="inset" action="/api/v1/commands/assignment.create" method="post" onSubmit={e=>{
  e.preventDefault();const form=new FormData(e.currentTarget);
  void action.run(async()=>{
   await command('assignment.create',{taskId:task.id,role:String(form.get('role')),input:String(form.get('input')),output:String(form.get('output')),writeScope:lines(String(form.get('scope'))),repoId:String(form.get('repo')),baseRef:String(form.get('base')),dependencies:form.getAll('dependencies').map(String)},undefined,base);
   onDone();
  });
 }}><fieldset disabled={!fresh||stale||action.busy}><legend>新增一份可認領的責任</legend>
  <Field label="分工角色" name="role" required maxLength={256}/><Area label="輸入約定" name="input" required maxLength={8192}/><Area label="交付約定" name="output" required maxLength={8192}/>
  <Area label="寫入範圍（每行一項）" name="scope"/>
  <Select label="Repository" name="repo" required>{project.repositories.map(r=><option key={r.id} value={r.id}>{r.name}</option>)}</Select>
  <Field label="基底參考" name="base" identifier defaultValue="main" required maxLength={256}/>
  {assignments.length?<fieldset><legend>需先完成的依賴</legend>{assignments.map(a=><Toggle key={a.id} name="dependencies" value={a.id} label={a.role}/>)}</fieldset>:null}
  <button type="submit">登記分工</button>
 </fieldset>{stale?<p className="notice">任務版本已變更，請關閉並重新開啟分工表單。</p>:null}<Feedback error={action.error}/></form>;
}
