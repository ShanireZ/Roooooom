import {useState} from 'react';
import {command,type Schema} from '../../api/client';
import {useCollection,useResource} from '../../api/resource';
import {navigate} from '../../App';
import {useI18n} from '../../i18n/I18nProvider';
import {localizeSystemQuestion} from '../decision/systemQuestions';
import {routePath} from '../../shell/routes';
import {Feedback} from '../../ui/Feedback';
import {Area,Select} from '../../ui/Field';
import {TechnicalDetails} from '../../ui/Fold';
import {RelativeTime} from '../../ui/RelativeTime';
import {Stale} from '../../ui/Stale';
import {Stamp} from '../../ui/Stamp';
import {useAction} from '../../ui/useAction';
import {useVersionedForm} from '../../ui/useVersionedForm';
import {Drawer} from '../Drawer';
import {List} from '../task/OverviewTab';
import {useNames} from '../task/TaskDrawer';

type WorkRequest=Schema['WorkRequest'];
// Durable human intent before it becomes a task; provenance afterwards.
export function RequestDrawer({projectId,requestId}:{projectId:string;requestId:string}){
 const {t}=useI18n();
 const record=useResource<WorkRequest>('/api/v1/queries/request?targetId='+encodeURIComponent(requestId));
 const sessions=useCollection<Schema['SessionView']>('/api/v1/queries/sessions?projectId='+encodeURIComponent(projectId));
 const questions=useCollection<Schema['DecisionRequestView']>('/api/v1/queries/decisions?targetId='+encodeURIComponent(requestId));
 const {names}=useNames(projectId);
 const close=()=>navigate(routePath({name:'room',projectId}));
 const r=record.data;
 if(!r)return <Drawer title={record.error?t('request.notFound'):t('task.loading')} onClose={close}><Feedback error={record.error}/></Drawer>;
 const session=sessions.data?.find(s=>s.id===r.preparerSessionId);
 const open=r.state==='waiting'||r.state==='preparing';
 return <Drawer title={r.text} onClose={close}>
  <div className="drawer-summary"><span className="label">{t(('request.state.'+r.state) as 'request.state.waiting')}</span></div>
  {r.taskId?<p><a href={routePath({name:'room',projectId,drawer:{kind:'task',id:r.taskId,tab:'overview'}})} data-local>{t('request.openTask')}</a></p>:null}
  {r.preparerParticipantId?<p>{t('request.preparer')}: {names[r.preparerParticipantId]??t('seat.revoked')}{session?<> · {session.status==='ended'?t('seat.ended'):<>{t('shell.lastSync')} <RelativeTime iso={session.lastSeenAt}/></>}</>:null}</p>
   :open?<p className="muted">{t('request.waiting')}</p>:null}
  {r.designatedIntegratorId?<p className="muted">{t('composer.integrator')}: {names[r.designatedIntegratorId]??r.designatedIntegratorId.slice(0,8)}</p>:null}
  {r.constraints?.length?<section className="drawer-section"><h3>{t('task.constraints')}</h3><List items={r.constraints}/></section>:null}
  {questions.data?.length?<section className="drawer-section"><h3>{t('request.questions')}</h3><ul className="plain-list">{questions.data.map(q=><li key={q.id}><Stamp kind="formal"/> <a href={routePath({name:'room',projectId,drawer:{kind:'decision',id:q.id}})} data-local>{localizeSystemQuestion(q,t).question}</a></li>)}</ul></section>:null}
  {open?<details className="fold"><summary>{t('request.manage')}</summary><RequestControls request={r} sessions={sessions.data??[]} names={names} fresh={record.fresh&&sessions.fresh}/></details>:null}
  <TechnicalDetails><pre>{JSON.stringify(r,null,2)}</pre></TechnicalDetails>
 </Drawer>;
}

function RequestControls({request,sessions,names,fresh}:{request:WorkRequest;sessions:Schema['SessionView'][];names:Record<string,string>;fresh:boolean}){
 const {t}=useI18n(),action=useAction(),[target,setTarget]=useState('');
 const form=useVersionedForm(request,x=>x.version),{base}=form;
 return <form onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget),assign=((e.nativeEvent as SubmitEvent).submitter as HTMLButtonElement|null)?.value==='assign';void action.run(async()=>{
  await command(assign?'request.assign':'request.cancel',{reason:String(data.get('reason')),...(assign?{toSessionId:target,expectedSourceSessionId:base.preparerSessionId??''}:{})},request.id,{expectedVersion:base.version,claimEpoch:base.preparationEpoch});
 })}}>
  <p className="muted">{t('request.manageHelp')}</p>
  <fieldset disabled={!fresh||form.stale||action.busy}>
   <Select label={t('request.newPreparer')} name="target" value={target} onChange={e=>setTarget(e.target.value)}><option value="">{t('request.noTarget')}</option>
    {sessions.filter(s=>s.status==='active'&&s.id!==base.preparerSessionId).map(s=><option key={s.id} value={s.id}>{names[s.participantId]??s.participantId.slice(0,8)} · {s.id.slice(0,8)}</option>)}</Select>
   <Area label={t('request.reason')} name="reason" required maxLength={8192}/>
   <button type="submit" value="assign" disabled={!target}>{t('request.confirmAssign')}</button>
   <button type="submit" value="cancel" className="btn-danger">{t('request.confirmCancel')}</button>
  </fieldset>
  <Stale stale={form.stale} onReopen={form.reopen}/>
  <Feedback error={action.error}/>
 </form>;
}
