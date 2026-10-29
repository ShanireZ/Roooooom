import {useState} from 'react';
import {ApiError,command,type Schema} from '../api/client';
import {useWorkspaceEvents} from '../api/events';
import {useCollection} from '../api/resource';
import {useI18n} from '../i18n/I18nProvider';
import {useRooms} from '../shell/Shell';
import {routePath} from '../shell/routes';
import {Feedback} from '../ui/Feedback';
import {Field} from '../ui/Field';
import {Fold} from '../ui/Fold';
import {useAction} from '../ui/useAction';
import {JoinInstructions} from './JoinInstructions';
import {readyForRetry} from './readyForRetry';

type Project=Schema['ProjectView'];
const suggestedName=(reference:string)=>reference.trim().replace(/[\\/]+$/,'').split(/[\\/]/).pop()?.replace(/\.git$/,'').slice(0,256)??'';

// First use on one screen: reference, name, open the door, then the instructions.
// Integrations never appear here; they live in room settings.
export function NewRoom(){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents(),{refresh}=useRooms();
 const [reference,setReference]=useState(''),[name,setName]=useState(''),[named,setNamed]=useState(false),[extra,setExtra]=useState<string[]>([]);
 const [project,setProject]=useState<Project>(),[open,setOpen]=useState(false),[uncertain,setUncertain]=useState(false);
 const openDoor=async(p:Project)=>{await command('enrollment.enable',{},p.id);setOpen(true);refresh()};
 if(project&&open)return <section className="card setup"><p className="label">{t('setup.eyebrow')}</p><h1>{project.name}</h1><JoinInstructions project={project}/><FirstAgent projectId={project.id}/>
  <a className="btn btn-ghost" href={routePath({name:'room',projectId:project.id})} data-local>{t('setup.goRoom')}</a></section>;
 return <section className="card setup">
  <p className="label">{t('setup.eyebrow')}</p>
  <h1>{t('setup.title')}</h1>
  <form onSubmit={e=>{e.preventDefault();void action.run(async()=>{
   if(!events.connected)await readyForRetry();
   if(project){await openDoor(project);return}
   const refs=[reference,...extra].map(x=>x.trim()).filter(Boolean);
   let created;
   try{created=await command<Project>('project.create',{name:name.trim(),repositories:refs.map((ref,i)=>({name:i===0?name.trim():suggestedName(ref),reference:ref}))})}
   catch(error){if(error instanceof ApiError&&error.status>=500)setUncertain(true);throw error}
   setUncertain(false);setProject(created.result);refresh();
   await openDoor(created.result);
  })}}>
   <fieldset disabled={action.busy||!!project}>
    <Field label={t('setup.reference')} name="reference" identifier value={reference} maxLength={2048} required readOnly={uncertain} placeholder={t('setup.referencePlaceholder')} onChange={e=>{setReference(e.target.value);if(!named)setName(suggestedName(e.target.value))}}/>
    <Field label={t('setup.name')} name="name" value={name} required maxLength={256} readOnly={uncertain} onChange={e=>{setNamed(true);setName(e.target.value)}}/>
    <p className="muted">{t('setup.nameHint')}</p>
    <Fold summary={t('setup.moreRepos')}>
     {extra.map((ref,i)=><Field key={i} label={t('setup.extraRepo',{n:i+1})} value={ref} identifier maxLength={2048} readOnly={uncertain} onChange={e=>setExtra(xs=>xs.map((x,n)=>n===i?e.target.value:x))}/>)}
     <button type="button" className="btn-ghost" disabled={extra.length>=19||uncertain} onClick={()=>setExtra(xs=>[...xs,''])}>{t('setup.addRepo')}</button>
    </Fold>
    <p className="muted">{t('setup.noScan')}</p>
    <p className="notice">{t('door.consent',{name:name||t('setup.thisRoom')})}</p>
   </fieldset>
   {uncertain?<p className="notice">{t('setup.uncertain')}</p>:null}
   {project&&!open?<p className="notice">{t('setup.registeredClosed')}</p>:null}
   <button type="submit" disabled={action.busy||!name.trim()||!reference.trim()}>{uncertain?t('setup.retry'):t('setup.submit')}</button>
   <Feedback error={action.error}/>
  </form>
 </section>;
}

// Updates live when the first agent synchronizes; Roooooom never starts or wakes agents.
export function FirstAgent({projectId}:{projectId:string}){
 const {t}=useI18n();
 const people=useCollection<Schema['ParticipantView']>('/api/v1/queries/participants?projectId='+encodeURIComponent(projectId));
 const first=people.data?.[0];
 return <p aria-live="polite" className={first?'success':'muted'}>{first?t('setup.joined',{name:first.name}):<><span className="pulse" aria-hidden="true"/>{t('setup.waiting')}</>} <span className="muted">{t('setup.noWake')}</span></p>;
}
