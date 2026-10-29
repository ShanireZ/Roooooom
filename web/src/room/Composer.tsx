import {forwardRef,useImperativeHandle,useRef,useState} from 'react';
import {ApiError,command,type Schema} from '../api/client';
import {useWorkspaceEvents} from '../api/events';
import {useI18n} from '../i18n/I18nProvider';
import {Feedback} from '../ui/Feedback';
import {Area,Field,Select,Toggle} from '../ui/Field';
import {Fold} from '../ui/Fold';
import {lines} from '../ui/text';
import {useAction} from '../ui/useAction';
import {submitRequest,type RequestSubmitState} from './requestSubmit';

type Participant=Schema['ParticipantView'];
export type ComposerHandle={ask():void};
const label=(p:Participant,all:Participant[])=>p.name+(all.filter(x=>x.name===p.name).length>1?' · '+p.id.slice(0,8):'');

// Two modes: ask agents to do something (a request) or say something (chat).
export const Composer=forwardRef<ComposerHandle,{projectId:string;participants:Participant[];onCreateTask?:()=>void}>(function Composer({projectId,participants,onCreateTask},ref){
 const {t}=useI18n(),[mode,setMode]=useState<'ask'|'say'>('ask'),askText=useRef<HTMLTextAreaElement>(null);
 useImperativeHandle(ref,()=>({ask(){setMode('ask');requestAnimationFrame(()=>askText.current?.focus())}}),[]);
 return <section className="composer" aria-label={t('composer.title')}>
  <div className="segmented" role="group" aria-label={t('composer.title')}>
   <button type="button" className="btn-ghost" aria-pressed={mode==='ask'} onClick={()=>setMode('ask')}>{t('composer.ask')}</button>
   <button type="button" className="btn-ghost" aria-pressed={mode==='say'} onClick={()=>setMode('say')}>{t('composer.say')}</button>
  </div>
  {mode==='ask'?<AskForm projectId={projectId} participants={participants} textRef={askText} onCreateTask={onCreateTask}/>:<ChatForm projectId={projectId}/>}
 </section>;
});

function AskForm({projectId,participants,textRef,onCreateTask}:{projectId:string;participants:Participant[];textRef:React.RefObject<HTMLTextAreaElement|null>;onCreateTask?:()=>void}){
 const {t}=useI18n(),events=useWorkspaceEvents(),action=useAction();
 const state=useRef<RequestSubmitState>({});
 const [uncertain,setUncertain]=useState(false),[text,setText]=useState(''),[conditions,setConditions]=useState('');
 return <form onSubmit={e=>{
  e.preventDefault();
  const form=e.currentTarget,data=new FormData(form);
  setUncertain(true);
  void action.run(async()=>{
   try{
    await submitRequest({projectId,text,constraints:lines(data.get('constraints')),collaboratorLimit:data.get('limit')?Number(data.get('limit')):undefined,
     designatedIntegratorId:String(data.get('integrator')??'')||undefined,acceptanceConditions:lines(conditions),acceptanceMode:data.get('acceptor')==='human'?'human':'integrator'},state.current);
    setUncertain(false);setText('');setConditions('');form.reset();action.setNotice(t('composer.asked'));
   }catch(error){if(error instanceof ApiError&&error.status<500)setUncertain(false);throw error}
  });
 }}>
  <fieldset disabled={!events.connected||action.busy}>
   <div className="field"><label htmlFor="composer-ask">{t('composer.askLabel')}</label>
    <textarea id="composer-ask" ref={textRef} name="text" value={text} onChange={e=>setText(e.target.value)} readOnly={uncertain} required maxLength={8192}/></div>
   <Fold summary={t('composer.more')}>
    <fieldset disabled={uncertain}>
     <Area label={t('composer.constraints')} name="constraints"/>
     <Field label={t('composer.limit')} name="limit" type="number" min={0} max={64}/>
     <Select label={t('composer.integrator')} name="integrator" defaultValue=""><option value="">{t('composer.anyIntegrator')}</option>{participants.map(p=><option key={p.id} value={p.id}>{label(p,participants)}</option>)}</Select>
     <p className="muted">{t('composer.integratorHint')}</p>
     <Area label={t('composer.conditions')} name="conditions" value={conditions} onChange={e=>setConditions(e.target.value)}/>
     {conditions.trim()?<Select label={t('composer.acceptor')} name="acceptor" defaultValue="integrator"><option value="integrator">{t('acceptor.integrator')}</option><option value="human">{t('acceptor.human')}</option></Select>:null}
    </fieldset>
   </Fold>
   {uncertain&&action.error?<p className="notice">{t('composer.uncertain')}</p>:null}
   <div className="composer-actions">
    <button type="submit">{uncertain&&action.error?t('composer.retry'):t('composer.askSubmit')}</button>
    {onCreateTask?<button type="button" className="btn-text" onClick={onCreateTask}>{t('composer.fullSpec')}</button>:null}
   </div>
  </fieldset>
  <Feedback error={action.error} notice={action.notice}/>
 </form>;
}

// Chat is discussion, never a formal decision; handling requests are explicit.
export function ChatForm({projectId,task}:{projectId:string;task?:Schema['Task']}){
 const {t}=useI18n(),events=useWorkspaceEvents(),action=useAction(),[body,setBody]=useState('');
 return <form onSubmit={e=>{
  e.preventDefault();
  const data=new FormData(e.currentTarget),actionRequired=data.get('actionRequired')==='on';
  void action.run(async()=>{
   await command('message.send',{projectId,taskId:task?.id,threadId:task?'task':'project',purpose:String(data.get('purpose')??'discussion'),priority:'normal',body,
    recipientMode:actionRequired&&task?.integratorId?'integrator':String(data.get('recipient')??'project'),recipientIds:[],actionRequired});
   setBody('');
  });
 }}>
  <fieldset disabled={!events.connected||action.busy}>
   <Area label={t('composer.sayLabel')} name="body" value={body} onChange={e=>setBody(e.target.value)} required maxLength={16384}/>
   <Toggle label={task?.integratorId?t('composer.handleTask'):t('composer.handleRoom')} name="actionRequired"/>
   <Fold summary={t('composer.routing')}>
    <Select label={t('composer.purpose')} name="purpose" defaultValue="discussion">{(['discussion','question','proposal','status','contract-change'] as const).map(p=><option key={p} value={p}>{t(('purpose.'+p) as 'purpose.discussion')}</option>)}</Select>
    <Select label={t('composer.recipient')} name="recipient" defaultValue="project"><option value="project">{t('recipient.project')}</option>{task?.integratorId?<option value="integrator">{t('recipient.integrator')}</option>:null}</Select>
   </Fold>
   <button type="submit">{t('composer.saySubmit')}</button>
  </fieldset>
  <Feedback error={action.error}/>
 </form>;
}
