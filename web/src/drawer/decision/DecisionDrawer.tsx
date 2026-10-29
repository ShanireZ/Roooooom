import {useEffect,useId,useMemo,useRef,useState} from 'react';
import {ApiError,command,type Schema} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {useResource} from '../../api/resource';
import {navigate} from '../../App';
import {useI18n} from '../../i18n/I18nProvider';
import {routePath} from '../../shell/routes';
import {useRooms} from '../../shell/Shell';
import {needsYouPath,useNeedsYou,type NeedsYouItem} from '../../shell/useNeedsYou';
import {Feedback} from '../../ui/Feedback';
import {Area,Field,Select,Toggle} from '../../ui/Field';
import {Fold,TechnicalDetails} from '../../ui/Fold';
import {RelativeTime} from '../../ui/RelativeTime';
import {Stale} from '../../ui/Stale';
import {Stamp} from '../../ui/Stamp';
import {lines} from '../../ui/text';
import {useAction} from '../../ui/useAction';
import {useVersionedForm} from '../../ui/useVersionedForm';
import {Drawer} from '../Drawer';
import {List} from '../task/OverviewTab';
import {useNames} from '../task/TaskDrawer';
import {approveAlso} from './approveAlso';
import {localizeSystemQuestion,type LocalizedQuestion} from './systemQuestions';

type Question=Schema['DecisionRequestView'];
type Detail={request:Question;history:(Schema['DecisionView']&{externalAuthorization?:unknown})[]};

// The formal question drawer (spec §8.4). Only the authenticated owner answers here;
// chat, relays and proposals never stand in for this form.
export function DecisionDrawer({projectId,decisionId}:{projectId:string;decisionId:string}){
 const {t}=useI18n();
 const detail=useResource<Detail>('/api/v1/queries/decision?targetId='+encodeURIComponent(decisionId));
 const q=detail.data?.request;
 const intake=useResource<Schema['WorkRequest']>('/api/v1/queries/request?targetId='+encodeURIComponent(q?.requestId??''),!!q?.requestId);
 const taskId=q?.taskId||intake.data?.taskId;
 const task=useResource<Schema['Task']>('/api/v1/queries/task?targetId='+encodeURIComponent(taskId??''),!!taskId);
 const {names}=useNames(projectId),{rooms}=useRooms();
 const needs=useNeedsYou(useMemo(()=>rooms.map(r=>r.id),[rooms]));
 const [editing,setEditing]=useState<'resolve'|'supersede'|undefined>(),[saved,setSaved]=useState('');
 const initialized=useRef(false);
 useEffect(()=>{if(q&&!initialized.current){initialized.current=true;if(q.state==='pending')setEditing('resolve')}},[q]);
 const close=()=>navigate(routePath({name:'room',projectId}));
 if(!q)return <Drawer title={detail.error?t('decision.notFound'):t('task.loading')} onClose={close}><Feedback error={detail.error}/></Drawer>;
 const L=localizeSystemQuestion(q,t);
 const author=q.actorKind==='system'?t('activity.system'):names[q.actorId]??t('seat.revoked');
 const recommended=L.options.find(o=>o.id===q.recommendation);
 const current:NeedsYouItem={kind:'decision',projectId,decisionRequestId:q.id,taskId:q.taskId||undefined,requestId:q.requestId||undefined,summary:q.question,since:q.receivedAt};
 const next=saved?needs.next(current):undefined;
 return <Drawer title={L.question} onClose={close}>
  <p><Stamp kind="formal"/> <span className="muted">{t('decision.onlyYou')}</span></p>
  {taskId&&task.data?<p><a href={routePath({name:'room',projectId,drawer:{kind:'task',id:taskId,tab:'overview'}})} data-local>{task.data.goal}</a></p>
   :q.requestId&&intake.data?<p><a href={routePath({name:'room',projectId,drawer:{kind:'request',id:q.requestId}})} data-local>{intake.data.text}</a></p>:null}
  <p>{L.reason}</p>
  {recommended?<p className="recommendation"><strong>{t('decision.recommends',{agent:author,option:recommended.label})}</strong> {L.recommendationReason}</p>:null}
  {detail.data?.history.map(d=><div key={d.id} className="formal-answer" data-testid="formal-decision">
   <p className="label">{t('decision.formalAnswer',{version:d.version})}</p><pre>{d.answer}</pre><p className="muted">{d.outcome} · <RelativeTime iso={d.receivedAt}/></p>
  </div>)}
  {q.followUpRequired&&!q.followUpComplete?<p className="notice">{t('decision.followupPending')}</p>:null}
  {saved?<div className="notice" role="status"><span>{saved}</span>{next?<button type="button" onClick={()=>navigate(needsYouPath(next))}>{t('decision.next')}</button>:<span className="muted"> {t('needsYou.empty')}</span>}</div>:null}
  {editing?<DecisionForm key={editing} question={q} localized={L} task={task.data} fresh={detail.fresh&&(!taskId||task.fresh)} mode={editing}
   onDone={message=>{setEditing(undefined);setSaved(message);detail.refresh()}}/>
   :q.state==='resolved'?<button type="button" className="btn-ghost" onClick={()=>{setSaved('');setEditing('supersede')}}>{t('decision.revise')}</button>:null}
 </Drawer>;
}

type Submitted={kind:string;payload:Record<string,unknown>;version:number};
function DecisionForm({question:q,localized:L,task,fresh,mode,onDone}:{question:Question;localized:LocalizedQuestion;task?:Schema['Task'];fresh:boolean;mode:'resolve'|'supersede';onDone:(message:string)=>void}){
 const {t}=useI18n(),events=useWorkspaceEvents(),action=useAction(),group=useId();
 const form=useVersionedForm(q,x=>x.version),also=approveAlso(q,task);
 const [option,setOption]=useState<string>(),[answer,setAnswer]=useState(''),[amend,setAmend]=useState(false),[authorize,setAuthorize]=useState(false),[withdrawing,setWithdrawing]=useState(false);
 const [uncertain,setUncertain]=useState<Submitted>();
 const visible=L.options.find(o=>o.id===option)?.label;
 const send=(s:Submitted)=>action.run(async()=>{
  try{
   await command(s.kind,s.payload,q.id,{expectedVersion:s.version});
   setUncertain(undefined);
   onDone(s.kind==='decision.withdraw'?t('decision.withdrawn'):t('decision.saved'));
  }catch(e){
   // An unknown result keeps the exact payload for an explicit retry; the server replays its receipt.
   if(!(e instanceof ApiError)||e.status>=500)setUncertain(s);
   throw e;
  }
 });
 return <form className="decision-form" onSubmit={e=>{
  e.preventDefault();
  const data=new FormData(e.currentTarget),withdraw=((e.nativeEvent as SubmitEvent).submitter as HTMLButtonElement|null)?.value==='withdraw';
  const formal=visible?visible+(answer.trim()?'\n'+t('decision.supplementPrefix')+answer.trim():''):answer.trim();
  const payload:Record<string,unknown>=withdraw?{reason:answer}:{answer:formal,optionId:option||undefined,supersedesDecisionId:mode==='supersede'?q.currentDecisionId:undefined,authorityFollowUpRequired:data.get('followup')==='on',outcome:option?undefined:String(data.get('outcome'))};
  if(amend&&!withdraw)payload.specAmendment={goal:String(data.get('amendGoal')),scope:lines(data.get('amendScope')),nonScope:lines(data.get('amendNonScope')),authorityRefs:lines(data.get('amendAuthority')),risk:String(data.get('amendRisk'))};
  if(authorize&&!withdraw)payload.externalAuthorization={action:String(data.get('externalAction')),target:String(data.get('externalTarget')),scope:String(data.get('externalScope')),originalText:String(data.get('externalText')),expiresAt:new Date(String(data.get('expiresAt'))).toISOString(),maxUses:Number(data.get('maxUses'))};
  void send({kind:withdraw?'decision.withdraw':mode==='resolve'?'decision.resolve':'decision.supersede',payload,version:form.base.version});
 }}>
  <fieldset disabled={!fresh||form.stale||action.busy||!!uncertain}>
   <legend>{mode==='resolve'?t('decision.yourAnswer'):t('decision.reviseTitle')}</legend>
   {L.options.map(o=><label key={o.id} className="choice option-card" htmlFor={group+o.id}>
    <input id={group+o.id} type="radio" name={group} value={o.id} checked={option===o.id} onChange={()=>setOption(o.id)}/>
    <span><strong>{o.label}</strong>{o.id===q.recommendation?<span className="rec-tag">{t('decision.recommended')}</span>:null}<small>{o.impact}</small></span>
   </label>)}
   <label className="choice option-card" htmlFor={group+'other'}><input id={group+'other'} type="radio" name={group} checked={option===''} onChange={()=>setOption('')}/><span><strong>{t('decision.other')}</strong><small>{t('decision.otherHelp')}</small></span></label>
   {option===''?<><Select label={t('decision.outcome')} name="outcome" defaultValue="answered">{(['answered','approved','rejected','revise'] as const).map(x=><option key={x} value={x}>{t(('decision.outcome.'+x) as 'decision.outcome.answered')}</option>)}</Select>
    <Area label={withdrawing?t('decision.withdrawReason'):t('decision.answerLabel')} name="answer" value={answer} onChange={e=>setAnswer(e.target.value)} required maxLength={8192}/></>
    :withdrawing?<Area label={t('decision.withdrawReason')} name="answer" value={answer} onChange={e=>setAnswer(e.target.value)} required maxLength={8192}/>
    :<Fold summary={t('decision.supplement')}><Area label={t('decision.supplementLabel')} name="answer" value={answer} onChange={e=>setAnswer(e.target.value)} maxLength={8192}/></Fold>}
   <Fold summary={t('decision.basis')}>
    <h4>{t('decision.facts')}</h4><List items={q.factRefs}/>
    <p>{t('decision.impact')}: {q.impact}</p>
    <h4>{t('decision.blocked')}</h4><List items={[...q.blockedActions,...q.blockedAssignmentIds]}/>
    <h4>{t('decision.canContinue')}</h4><List items={L.canContinue}/>
    <p className="muted">{t('decision.ifUnanswered')}: {L.noDecisionBehavior}{q.deadline?' · '+t('decision.deadline'):''}{q.deadline?<> <RelativeTime iso={q.deadline}/></>:null}</p>
    <Toggle label={t('decision.followup')} name="followup"/>
   </Fold>
   {also.spec||also.external?<section className="approve-also">
    <h3>{t('decision.alsoTitle')}</h3>
    {also.spec?<div>
     {also.spec.proposed?<p className="muted">{t('decision.proposedSpec')} · {t('decision.notApproved')}</p>:null}
     <Toggle label={t('decision.amendSpec')} checked={amend} onChange={e=>setAmend(e.target.checked)}/>
     {amend?<div className="inset">
      <Field label={t('decision.amendGoal')} name="amendGoal" defaultValue={also.spec.prefill.goal} required/>
      <Area label={t('create.scope')} name="amendScope" defaultValue={also.spec.prefill.scope.join('\n')} required/>
      <Area label={t('create.nonScope')} name="amendNonScope" defaultValue={also.spec.prefill.nonScope.join('\n')}/>
      <Area label={t('create.authority')} name="amendAuthority" defaultValue={also.spec.prefill.authorityRefs.join('\n')} required/>
      <Select label={t('create.risk')} name="amendRisk" defaultValue={also.spec.prefill.risk}><option value="low">{t('risk.low')}</option><option value="normal">{t('risk.normal')}</option><option value="high">{t('risk.high')}</option></Select>
     </div>:null}
    </div>:null}
    {also.external?<div>
     {also.external.proposed?<p className="notice">{t('decision.proposedExternal',{action:also.external.prefill.action??'',target:also.external.prefill.target??''})} · {also.external.prefill.scope} · {t('decision.notApproved')}</p>:null}
     <Toggle label={t('decision.authorizeExternal')} checked={authorize} onChange={e=>setAuthorize(e.target.checked)}/>
     {authorize?<div className="inset">
      <p className="muted">{t('decision.externalNote')}</p>
      <Select label={t('decision.externalAction')} name="externalAction" defaultValue={also.external.prefill.action} required>{['push','pr','merge','release','deploy'].map(x=><option key={x}>{x}</option>)}</Select>
      <Field label={t('decision.externalTarget')} name="externalTarget" defaultValue={also.external.prefill.target} identifier required/>
      <Area label={t('decision.externalScope')} name="externalScope" defaultValue={also.external.prefill.scope} required/>
      <Area label={t('decision.externalText')} name="externalText" defaultValue={also.external.prefill.originalText} required/>
      <Field label={t('decision.externalExpires')} name="expiresAt" type="datetime-local" defaultValue={also.external.prefill.expiresAt?new Date(new Date(also.external.prefill.expiresAt).getTime()-new Date().getTimezoneOffset()*60000).toISOString().slice(0,16):undefined} required/>
      <Field label={t('decision.externalUses')} name="maxUses" type="number" min={1} max={100} defaultValue={also.external.prefill.maxUses??1} required/>
     </div>:null}
    </div>:null}
   </section>:null}
   <TechnicalDetails><p>{q.id} · v{q.version} · {q.scopeKey} · {q.state}</p><List items={q.blockedAssignmentIds}/></TechnicalDetails>
   <div className="actions">{withdrawing?<>
    <button type="submit" name="action" value="withdraw" className="btn-danger">{t('decision.confirmWithdraw')}</button>
    <button type="button" className="btn-ghost" onClick={()=>setWithdrawing(false)}>{t('decision.backToOptions')}</button>
   </>:<>
    <button type="submit" className="btn-owner" disabled={option===undefined}>{t('decision.confirmPrefix')+(option===undefined?t('decision.chooseFirst'):visible??t('decision.other'))}</button>
    {mode==='resolve'?<button type="button" className="btn-ghost" onClick={()=>{setWithdrawing(true);setAnswer('')}}>{t('decision.withdraw')}</button>:null}
   </>}</div>
  </fieldset>
  {uncertain?<p className="notice">{t('decision.uncertain')} <button type="button" disabled={!events.connected||action.busy} onClick={()=>void send(uncertain)}>{t('decision.retry')}</button></p>:null}
  {!uncertain?<Stale stale={form.stale} onReopen={form.reopen}/>:null}
  <Feedback error={action.error}/>
 </form>;
}
