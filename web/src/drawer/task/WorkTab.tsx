import {useState} from 'react';
import {command,type Schema} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {useCollection,useResource} from '../../api/resource';
import {useI18n} from '../../i18n/I18nProvider';
import {Feedback} from '../../ui/Feedback';
import {Area,Field,Select,Toggle} from '../../ui/Field';
import {TechnicalDetails} from '../../ui/Fold';
import {Stale} from '../../ui/Stale';
import {lines} from '../../ui/text';
import {useAction} from '../../ui/useAction';
import {useVersionedForm} from '../../ui/useVersionedForm';
import {List} from './OverviewTab';
import type {TaskView} from './TaskDrawer';
import {versions} from './versions';

type Assignment=Schema['Assignment'];
const closed=(task:TaskView)=>task.phase==='done'||task.phase==='cancelled';

// Responsibilities split from the task; one valid claim each.
export function WorkTab({task,names,fresh}:{task:TaskView;names:Record<string,string>;fresh:boolean}){
 const {t}=useI18n(),[adding,setAdding]=useState(false);
 const rows=useCollection<Assignment>('/api/v1/queries/assignments?targetId='+encodeURIComponent(task.id));
 return <>
  {!task.integratorId?<p className="muted">{t('work.needsIntegrator')}</p>:null}
  <ul className="record-list" aria-label={t('task.tab.work')}>
   {rows.data?.map(a=><li key={a.id}>
    <p><strong>{a.role}</strong> · {t(('assignment.state.'+a.state) as 'assignment.state.open')}</p>
    <p>{a.output}</p>
    <p className="muted">{a.claimParticipantId?names[a.claimParticipantId]??t('seat.revoked'):t('work.open')} · {a.canStart?t('work.canStart'):t('work.cannotStart')}</p>
    {a.cancellationReason?<p>{a.cancellationReason}</p>:null}
    <TechnicalDetails><p>{t('work.input')}: {a.input}</p><List items={a.writeScope}/><p>{a.repoId} @ {a.baseRef}</p><List items={a.dependencies}/><p>contract v{a.contractVersion} · claim epoch {a.claimEpoch} · v{a.version}</p></TechnicalDetails>
    {!closed(task)&&a.state!=='accepted'&&a.state!=='cancelled'?<CancelAssignment task={task} assignment={a} fresh={fresh&&rows.fresh}/>:null}
   </li>)}
  </ul>
  {!rows.data?.length&&!adding?<p className="muted">{t('work.empty')}</p>:null}
  {task.integratorId&&!closed(task)?(adding?<AssignmentForm task={task} assignments={rows.data??[]} fresh={fresh} onDone={()=>setAdding(false)}/>:<button type="button" className="btn-ghost" disabled={!fresh} onClick={()=>setAdding(true)}>{t('work.add')}</button>):null}
 </>;
}

function CancelAssignment({task,assignment,fresh}:{task:TaskView;assignment:Assignment;fresh:boolean}){
 const {t}=useI18n(),[open,setOpen]=useState(false);
 return open?<CancelForm task={task} assignment={assignment} fresh={fresh} onClose={()=>setOpen(false)}/>:<button type="button" className="btn-ghost" disabled={!fresh} onClick={()=>setOpen(true)}>{t('work.cancel')}</button>;
}
function CancelForm({task,assignment,fresh,onClose}:{task:TaskView;assignment:Assignment;fresh:boolean;onClose:()=>void}){
 const {t}=useI18n(),action=useAction();
 const form=useVersionedForm({task,assignment},x=>x.task.version*1e6+x.assignment.version);
 const {base}=form;
 return <form onSubmit={e=>{e.preventDefault();const reason=String(new FormData(e.currentTarget).get('reason'));void action.run(async()=>{
  await command('assignment.update',{state:'cancelled',reason},assignment.id,{...versions(base.task),expectedVersion:base.assignment.version,claimEpoch:base.assignment.claimEpoch});onClose();
 })}}>
  <p className="muted">{t('work.cancelHelp')}</p>
  <fieldset disabled={!fresh||form.stale||action.busy}>
   <Area label={t('work.cancelReason')} name="reason" required maxLength={8192}/>
   <button type="submit" className="btn-danger">{t('work.confirmCancel')}</button>
   <button type="button" className="btn-ghost" onClick={onClose}>{t('form.close')}</button>
  </fieldset>
  <Stale stale={form.stale} onReopen={form.reopen}/>
  <Feedback error={action.error}/>
 </form>;
}

function AssignmentForm({task,assignments,fresh,onDone}:{task:TaskView;assignments:Assignment[];fresh:boolean;onDone:()=>void}){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents();
 const form=useVersionedForm(task,x=>x.version);
 const project=useResource<Schema['ProjectView'][]>('/api/v1/queries/projects',true,true);
 const repos=project.data?.find(p=>p.id===task.projectId)?.repositories??[];
 return <form onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget);void action.run(async()=>{
  await command('assignment.create',{taskId:task.id,role:String(data.get('role')),input:String(data.get('input')),output:String(data.get('output')),writeScope:lines(data.get('scope')),repoId:String(data.get('repo')),baseRef:String(data.get('base')),dependencies:data.getAll('dependencies').map(String)},undefined,versions(form.base));
  onDone();
 })}}>
  <fieldset disabled={!fresh||!events.connected||form.stale||action.busy}>
   <legend>{t('work.newTitle')}</legend>
   <Field label={t('work.role')} name="role" required maxLength={256}/>
   <Area label={t('work.input')} name="input" required maxLength={8192}/>
   <Area label={t('work.output')} name="output" required maxLength={8192}/>
   <Area label={t('work.scope')} name="scope"/>
   <Select label={t('work.repo')} name="repo" required>{repos.map(r=><option key={r.id} value={r.id}>{r.name}</option>)}</Select>
   <Field label={t('work.base')} name="base" identifier defaultValue="main" required maxLength={256}/>
   {assignments.length?<fieldset><legend>{t('work.dependencies')}</legend>{assignments.map(a=><Toggle key={a.id} name="dependencies" value={a.id} label={a.role}/>)}</fieldset>:null}
   <button type="submit">{t('work.submit')}</button>
   <button type="button" className="btn-ghost" onClick={onDone}>{t('form.close')}</button>
  </fieldset>
  <Stale stale={form.stale} onReopen={form.reopen}/>
  <Feedback error={action.error}/>
 </form>;
}
