import {useState} from 'react';
import {command} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {useCollection} from '../../api/resource';
import {useI18n} from '../../i18n/I18nProvider';
import {readyForRetry} from '../../setup/readyForRetry';
import {Feedback} from '../../ui/Feedback';
import {Field,Select,Toggle} from '../../ui/Field';
import {Fold} from '../../ui/Fold';
import {useAction} from '../../ui/useAction';

type Binding={id:string;projectId:string;version:number;kind:string;repository:string;repositoryId:number;private:boolean;credentialRef:string;fields:string[];events:string[];enabled:boolean;needsRebind:boolean};
type Job={jobId:string;taskId:string;bindingId:string;version:number;kind:string;status:string;errorCode?:string;attempts:number;outcomeUncertain?:boolean};
type Inspection={repository:string;repositoryId:number;private:boolean;hasIssues:boolean;issuesWriteVerified:false};
const GITHUB_FIELDS=['taskId','goal','state','scope','acceptance'];
const GITHUB_EVENTS=['task.create','integration.record','task.complete','task.cancel','task.reopen','decision.request','decision.resolve','decision.supersede','delivery.submit','delivery.review','acceptance.candidate','acceptance.accept'];
const DEFAULT_EVENTS=['task.create','integration.record','task.complete','task.cancel','task.reopen'];
const DINGTALK_FIELDS=['taskId','goal','projectTitle','summary'];
const DINGTALK_EVENTS=['decision-pending','human-acceptance','owner-blocker'];
type Label=(key:string)=>string;

// Optional external records and notifications. Roooooom stays the task authority;
// credentials stay in private product data and nothing is test-sent on save.
export function IntegrationsSection({projectId}:{projectId:string}){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents();
 const bindings=useCollection<Binding>('/api/v1/queries/bindings?projectId='+encodeURIComponent(projectId));
 const jobs=useCollection<Job>('/api/v1/queries/outbound-jobs?projectId='+encodeURIComponent(projectId));
 const [credential,setCredential]=useState({kind:'',reference:''});
 const label:Label=key=>t(key as 'field.taskId');
 return <>
  <p className="muted">{t('integrations.githubHelp')}</p>
  <p className="muted">{t('integrations.credentialHelp')}</p>
  <Guided projectId={projectId} onEnabled={bindings.refresh}/>
  <ul className="plain-list">{bindings.data?.filter(b=>b.enabled).map(b=><li key={b.id}>{b.kind==='github'?'GitHub · '+b.repository:t('integrations.dingtalk')} · {b.needsRebind?t('bindings.needsRebind'):t('bindings.enabled')}</li>)}</ul>
  <Fold summary={t('advanced.title')}>
   <CredentialForm onSaved={(kind,reference)=>setCredential({kind,reference})}/>
   <form onSubmit={e=>{e.preventDefault();const el=e.currentTarget,data=new FormData(el);void action.run(async()=>{
    await command('binding.create',{projectId,kind:'github',repository:String(data.get('repository')),repositoryId:Number(data.get('repositoryId')),private:data.get('visibility')==='private',credentialRef:String(data.get('credentialRef')),fields:data.getAll('fields').map(String),events:data.getAll('events').map(String),enabled:true});
    el.reset();bindings.refresh();action.setNotice(t('advanced.githubCreated'));
   })}}><fieldset disabled={!events.connected||action.busy}><legend>{t('advanced.githubTitle')}</legend>
    <Field label={t('advanced.githubRepo')} name="repository" identifier required pattern="[A-Za-z0-9_-]+/[A-Za-z0-9_.-]+" maxLength={256}/>
    <Field label={t('advanced.repoId')} name="repositoryId" type="number" min={1} step={1} required/>
    <Select label={t('advanced.visibility')} name="visibility" defaultValue="private"><option value="private">{t('guided.private')}</option><option value="public">{t('guided.public')}</option></Select>
    <Field key={credential.kind==='github'?credential.reference:'pat'} label={t('advanced.patRef')} name="credentialRef" defaultValue={credential.kind==='github'?credential.reference:''} identifier required pattern="[a-zA-Z0-9_-]{1,128}" maxLength={128}/>
    <fieldset><legend>{t('advanced.fields')}</legend>{GITHUB_FIELDS.map(v=><Toggle key={v} label={label('field.'+v)} name="fields" value={v} defaultChecked/>)}</fieldset>
    <fieldset><legend>{t('advanced.events')}</legend>{GITHUB_EVENTS.map(v=><Toggle key={v} label={label('event.'+v)} name="events" value={v} defaultChecked/>)}</fieldset>
    <button type="submit">{t('advanced.createGithub')}</button>
   </fieldset></form>
   <form onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget);void action.run(async()=>{
    await command('binding.create',{projectId,kind:'dingtalk',credentialRef:String(data.get('credentialRef')),enabled:true,fields:data.getAll('fields').map(String),events:data.getAll('events').map(String)});
    bindings.refresh();action.setNotice(t('advanced.dingtalkCreated'));
   })}}><fieldset disabled={!events.connected||action.busy}><legend>{t('advanced.dingtalkTitle')}</legend>
    <Field key={credential.kind==='dingtalk'?credential.reference:'bot'} label={t('advanced.dingtalkRef')} name="credentialRef" defaultValue={credential.kind==='dingtalk'?credential.reference:''} identifier required pattern="[a-zA-Z0-9_-]{1,128}" maxLength={128}/>
    <fieldset><legend>{t('advanced.dingtalkFields')}</legend>{DINGTALK_FIELDS.map(v=><Toggle key={v} label={label('field.'+v)} name="fields" value={v} defaultChecked/>)}</fieldset>
    <fieldset><legend>{t('advanced.dingtalkEvents')}</legend>{DINGTALK_EVENTS.map(v=><Toggle key={v} label={label('event.'+v)} name="events" value={v} defaultChecked/>)}</fieldset>
    <p className="muted">{t('advanced.dingtalkLimits')}</p>
    <button type="submit">{t('advanced.createDingtalk')}</button>
   </fieldset></form>
   <ul className="record-list">{bindings.data?.map(b=><li key={b.id}>
    <strong className="mono">{b.kind} · {b.repository||b.credentialRef}</strong>
    <p className="muted">{b.needsRebind?t('bindings.needsRebind'):b.enabled?t('bindings.enabled'):t('bindings.disabled')} · v{b.version} · <span className="mono">{b.id}</span></p>
    <form onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget);void action.run(async()=>{await command('binding.rotate',{credentialRef:String(data.get('credentialRef')),reason:String(data.get('reason'))},b.id,{expectedVersion:b.version});bindings.refresh();action.setNotice(t('advanced.rotated'))})}}>
     <fieldset disabled={!b.enabled||b.needsRebind||!events.connected||action.busy}>
      <Field key={credential.reference} label={t('advanced.rotateRef')} name="credentialRef" defaultValue={credential.kind===b.kind?credential.reference:''} identifier required pattern="[a-zA-Z0-9_-]{1,128}"/>
      <Field label={t('advanced.rotateReason')} name="reason" required maxLength={8192}/>
      <button type="submit" className="btn-ghost">{t('advanced.rotate')}</button>
     </fieldset>
    </form>
    <button type="button" className="btn-ghost" disabled={!events.connected||action.busy||!b.enabled} onClick={()=>void action.run(async()=>{await command('binding.disable',{},b.id,{expectedVersion:b.version});bindings.refresh();action.setNotice(t('advanced.disabledNotice'))})}>{t('advanced.disable')}</button>
   </li>)}</ul>
   <Fold summary={t('advanced.jobs')}><ul className="record-list">{jobs.data?.map(j=><li key={j.jobId}>
    <span className="mono">{j.jobId}</span> · {j.kind} · {j.status} · {j.attempts}{j.errorCode?' · '+j.errorCode:''}
    {j.status==='uncertain'||j.status==='paused'?<form onSubmit={e=>{e.preventDefault();const reason=String(new FormData(e.currentTarget).get('reason'));void action.run(async()=>{await command('binding.retry',{mode:j.status==='uncertain'||(j.outcomeUncertain&&j.kind!=='dingtalk.notify')?'reconcile':'retry',reason},j.jobId,{expectedVersion:j.version});jobs.refresh();action.setNotice(t('advanced.retried'))})}}>
     <fieldset disabled={!events.connected||action.busy}><Field label={t('advanced.jobReason')} name="reason" required maxLength={8192}/><button type="submit">{j.status==='uncertain'?t('advanced.reconcile'):t('advanced.retry')}</button></fieldset>
    </form>:null}
   </li>)}</ul></Fold>
   <p className="muted">{t('advanced.switchNote')}</p>
  </Fold>
  <Feedback error={action.error??bindings.error??jobs.error} notice={action.notice}/>
 </>;
}

function Guided({projectId,onEnabled}:{projectId:string;onEnabled:()=>void}){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents();
 const [kind,setKind]=useState<''|'github'|'dingtalk'>(''),[reference,setReference]=useState(''),[secret,setSecret]=useState(''),[token,setToken]=useState(''),[ref,setRef]=useState('');
 const [inspection,setInspection]=useState<Inspection>(),[confirmed,setConfirmed]=useState(false),[done,setDone]=useState(false),[custom,setCustom]=useState<string[]>(DEFAULT_EVENTS);
 const select=(value:'github'|'dingtalk')=>{setKind(value);setReference('');setSecret('');setToken('');setRef('');setInspection(undefined);setConfirmed(false);setDone(false);setCustom(DEFAULT_EVENTS)};
 // The credential is stored once; a failed inspection resumes with the saved reference.
 const prepare=()=>void action.run(async()=>{
  if(!events.connected)await readyForRetry();
  let credential=ref;
  if(!credential){const r=await command<{credentialRef:string}>('binding.credential',{kind,...(kind==='github'?{token}:{webhookUrl:reference,secret})});credential=r.result.credentialRef;setRef(credential);setToken('');setSecret('')}
  if(kind==='github'){const r=await command<Inspection>('binding.inspect',{repository:reference,credentialRef:credential});setInspection(r.result)}
  setConfirmed(true);
 });
 const enable=()=>void action.run(async()=>{
  if(!events.connected)await readyForRetry();
  await command('binding.create',{projectId,kind,credentialRef:ref,enabled:true,...(kind==='github'?{repository:inspection!.repository,repositoryId:inspection!.repositoryId,private:inspection!.private,fields:GITHUB_FIELDS,events:custom}:{fields:['projectTitle','goal','summary'],events:DINGTALK_EVENTS})});
  setDone(true);setReference('');onEnabled();action.setNotice(kind==='github'?t('guided.githubEnabled'):t('guided.dingtalkEnabled'));
 });
 return <div className="guided">
  <div className="actions"><button type="button" className="btn-ghost" disabled={action.busy} onClick={()=>select('github')}>{t('guided.github')}</button><button type="button" className="btn-ghost" disabled={action.busy} onClick={()=>select('dingtalk')}>{t('guided.dingtalk')}</button></div>
  {kind&&!done?<section className="drawer-section"><h3>{kind==='github'?t('guided.githubTitle'):t('guided.dingtalkTitle')}</h3>
   {!confirmed?<form onSubmit={e=>{e.preventDefault();prepare()}}><fieldset disabled={action.busy}>
    <Field label={kind==='github'?t('guided.repoUrl'):t('guided.webhookUrl')} value={reference} onChange={e=>setReference(e.target.value)} readOnly={!!ref} required maxLength={2048} autoComplete="off"/>
    {!ref?kind==='github'?<Field label={t('guided.pat')} value={token} onChange={e=>setToken(e.target.value)} type="password" autoComplete="new-password" required maxLength={512}/>:<Field label={t('guided.secret')} value={secret} onChange={e=>setSecret(e.target.value)} type="password" autoComplete="new-password" required maxLength={512}/>:<p className="muted">{t('guided.credentialKept')}</p>}
    <p className="muted">{kind==='github'?t('guided.githubHint'):t('guided.dingtalkHint')}</p>
    <button type="submit">{kind==='github'?t('guided.confirmDestination'):t('guided.confirmScope')}</button>
   </fieldset></form>:<>
    <p>{kind==='github'?t('guided.destination',{repository:inspection!.repository,visibility:inspection!.private?t('guided.private'):t('guided.public')}):t('guided.dingtalkDestination')}</p>
    {kind==='github'?<><p>{t('guided.githubPlan')}</p><p className="notice">{t('guided.notVerified')}</p>
     <Fold summary={t('guided.customize')}>{DEFAULT_EVENTS.map(v=><Toggle key={v} label={t(('event.'+v) as 'event.task.create')} checked={custom.includes(v)} onChange={e=>setCustom(e.target.checked?[...custom,v]:custom.filter(x=>x!==v))}/>)}</Fold></>
     :<><p>{t('guided.dingtalkScope')}</p><p className="muted">{t('guided.languageNote')}</p></>}
    <p className="muted">{t('guided.formalOnLocal')}</p>
    <button type="button" disabled={action.busy} onClick={enable}>{kind==='github'?t('guided.enableGithub'):t('guided.enableDingtalk')}</button>
   </>}
  </section>:null}
  <Feedback error={action.error} notice={action.notice}/>
 </div>;
}

function CredentialForm({onSaved}:{onSaved:(kind:string,reference:string)=>void}){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents(),[kind,setKind]=useState('github');
 return <form onSubmit={e=>{e.preventDefault();const el=e.currentTarget,data=new FormData(el);void action.run(async()=>{
  const r=await command<{credentialRef:string}>('binding.credential',{kind,token:String(data.get('token')),...(kind==='dingtalk'?{secret:String(data.get('secret'))}:{})});
  el.reset();onSaved(kind,r.result.credentialRef);action.setNotice(t('advanced.credentialSaved'));
 })}}><fieldset disabled={!events.connected||action.busy}><legend>{t('advanced.credentialTitle')}</legend>
  <Select label={t('advanced.credentialKind')} value={kind} onChange={e=>setKind(e.target.value)}><option value="github">{t('advanced.githubPat')}</option><option value="dingtalk">{t('advanced.dingtalkWebhook')}</option></Select>
  <Field label={t('advanced.token')} name="token" type="password" required maxLength={512} autoComplete="new-password"/>
  {kind==='dingtalk'?<><p className="muted">{t('advanced.dingtalkTokenHint')}</p><Field label={t('guided.secret')} name="secret" type="password" required maxLength={512} autoComplete="new-password"/></>:null}
  <button type="submit">{t('advanced.saveCredential')}</button>
 </fieldset><Feedback error={action.error} notice={action.notice}/></form>;
}
