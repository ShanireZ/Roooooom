import {useState,type ReactNode} from 'react';
import {command,type Schema} from '../../api/client';
import {useCollection} from '../../api/resource';
import {useI18n} from '../../i18n/I18nProvider';
import {Feedback} from '../../ui/Feedback';
import {Area,Field,Select,Toggle} from '../../ui/Field';
import {Fold} from '../../ui/Fold';
import {Stale} from '../../ui/Stale';
import {lines} from '../../ui/text';
import {useAction} from '../../ui/useAction';
import {useVersionedForm} from '../../ui/useVersionedForm';
import type {TaskView} from './TaskDrawer';
import {versions} from './versions';

// Owner-only management. Every action states a reason and keeps today's version fences.
export function ManageTab({task,names,people,fresh}:{task:TaskView;names:Record<string,string>;people:Schema['ParticipantView'][];fresh:boolean}){
 const {t}=useI18n();
 return <>
  <Fold lazy summary={t('manage.limit')}><LimitForm task={task} fresh={fresh}/></Fold>
  <Fold lazy summary={t('manage.policy')}><PolicyEditor task={task} people={people} names={names} fresh={fresh}/></Fold>
  {task.integratorId&&task.phase!=='done'&&task.phase!=='cancelled'?<Fold lazy summary={t('manage.handoff')}><HandoffForm task={task} names={names} fresh={fresh}/></Fold>:null}
  {task.phase==='cancelled'?<p className="muted">{t('manage.cancelledNote')}</p>:<CloseForm task={task} fresh={fresh}/>}
 </>;
}

function Bound({task,fresh,children,onSubmit,action}:{task:TaskView;fresh:boolean;children:(base:TaskView)=>ReactNode;onSubmit:(data:FormData,base:TaskView)=>Promise<unknown>;action:ReturnType<typeof useAction>}){
 const form=useVersionedForm(task,x=>x.version);
 return <form onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget),el=e.currentTarget;void action.run(async()=>{await onSubmit(data,form.base);el.reset()})}}>
  <fieldset disabled={!fresh||form.stale||action.busy}>{children(form.base)}</fieldset>
  <Stale stale={form.stale} onReopen={form.reopen}/>
  <Feedback error={action.error} notice={action.notice}/>
 </form>;
}

function LimitForm({task,fresh}:{task:TaskView;fresh:boolean}){
 const {t}=useI18n(),action=useAction();
 return <Bound task={task} fresh={fresh} action={action} onSubmit={async(data,base)=>{await command('collaborator.limit',{limit:Number(data.get('limit')),reason:String(data.get('reason'))},task.id,versions(base));action.setNotice(t('manage.limitSaved'))}}>
  {base=><><Field label={t('manage.newLimit')} type="number" name="limit" min={base.collaboratorCount} max={64} defaultValue={base.collaboratorLimit} required/>
   <Field label={t('manage.limitReason')} name="reason" required/>
   <button type="submit">{t('manage.updateLimit')}</button></>}
 </Bound>;
}

function PolicyEditor({task,people,names,fresh}:{task:TaskView;people:Schema['ParticipantView'][];names:Record<string,string>;fresh:boolean}){
 const {t}=useI18n(),action=useAction(),[mode,setMode]=useState<string>(task.acceptancePolicy.mode);
 return <Bound task={task} fresh={fresh} action={action} onSubmit={async(data,base)=>{
  await command('acceptance.policy',{policy:{mode,acceptorId:mode==='designated-agent'?String(data.get('acceptor')):undefined,conditions:lines(data.get('conditions'))},reason:String(data.get('reason'))},task.id,versions(base));
  action.setNotice(t('manage.policySaved'));
 }}>
  {base=><><p className="muted">{t('manage.policyHelp')}</p>
   <Select label={t('create.acceptanceMode')} value={mode} onChange={e=>setMode(e.target.value)}><option value="integrator">{t('acceptor.integrator')}</option><option value="human">{t('acceptor.human')}</option><option value="designated-agent">{t('acceptor.designated')}</option></Select>
   {mode==='designated-agent'?<Select label={t('create.acceptor')} name="acceptor" defaultValue={base.acceptancePolicy.acceptorId} required><option value="">{t('form.choose')}</option>{people.map(p=><option key={p.id} value={p.id}>{names[p.id]}</option>)}</Select>:null}
   <Area label={t('create.conditions')} name="conditions" defaultValue={base.acceptancePolicy.conditions.join('\n')} required/>
   <Field label={t('manage.policyReason')} name="reason" required/>
   <button type="submit">{t('manage.updatePolicy')}</button></>}
 </Bound>;
}

function HandoffForm({task,names,fresh}:{task:TaskView;names:Record<string,string>;fresh:boolean}){
 const {t}=useI18n(),action=useAction(),[target,setTarget]=useState('');
 const sessions=useCollection<Schema['SessionView']>('/api/v1/queries/sessions?projectId='+encodeURIComponent(task.projectId));
 const assignments=useCollection<Schema['Assignment']>('/api/v1/queries/assignments?targetId='+encodeURIComponent(task.id));
 const claims=(assignments.data??[]).filter(a=>a.claimParticipantId===task.integratorId&&a.state!=='accepted'&&a.state!=='cancelled');
 return <Bound task={task} fresh={fresh&&sessions.fresh&&assignments.fresh} action={action} onSubmit={async(data,base)=>{
  const dispositions=claims.map(a=>{const d=String(data.get('disposition-'+a.id));return {assignmentId:a.id,action:d,targetSessionId:d==='release'?undefined:d==='continue-worker'?base.integratorSessionId:target}});
  await command('integrator.handoff',{fromSessionId:base.integratorSessionId,toSessionId:target,dispositions,reason:String(data.get('reason')),force:true},task.id,versions(base));
 }}>
  {()=><><p className="muted">{t('manage.handoffHelp')}</p>
   <Select label={t('manage.handoffTarget')} name="target" value={target} onChange={e=>setTarget(e.target.value)} required><option value="">{t('form.choose')}</option>
    {sessions.data?.filter(s=>s.status==='active'&&s.participantId!==task.integratorId).map(s=><option key={s.id} value={s.id}>{names[s.participantId]??s.participantId.slice(0,8)} · {s.id.slice(0,8)}</option>)}</Select>
   {claims.map(a=><Select key={a.id} label={t('manage.disposition',{role:a.role})} name={'disposition-'+a.id} defaultValue="" required><option value="">{t('form.choose')}</option><option value="transfer">{t('manage.transfer')}</option><option value="release">{t('manage.release')}</option><option value="continue-worker">{t('manage.continue')}</option></Select>)}
   <Field label={t('manage.handoffReason')} name="reason" required maxLength={8192}/>
   <Toggle label={t('manage.handoffConfirm')} name="confirm" required/>
   <button type="submit" className="btn-danger">{t('manage.confirmHandoff')}</button></>}
 </Bound>;
}

function CloseForm({task,fresh}:{task:TaskView;fresh:boolean}){
 const {t}=useI18n(),action=useAction(),[open,setOpen]=useState(false),done=task.phase==='done';
 if(!open)return <button type="button" className={done?'btn-ghost':'btn-danger'} disabled={!fresh} onClick={()=>setOpen(true)}>{done?t('manage.reopen'):t('manage.cancel')}</button>;
 return <Bound task={task} fresh={fresh} action={action} onSubmit={async(data,base)=>{await command(done?'task.reopen':'task.cancel',{reason:String(data.get('reason'))},task.id,versions(base));setOpen(false)}}>
  {()=><><Field label={done?t('manage.reopenReason'):t('manage.cancelReason')} name="reason" required/>
   <button type="submit" className={done?'':'btn-danger'}>{done?t('manage.confirmReopen'):t('manage.confirmCancel')}</button>
   <button type="button" className="btn-ghost" onClick={()=>setOpen(false)}>{t('form.close')}</button></>}
 </Bound>;
}
