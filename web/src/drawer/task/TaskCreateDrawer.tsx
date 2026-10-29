import {useState} from 'react';
import {command,type Schema} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {navigate} from '../../App';
import {useI18n} from '../../i18n/I18nProvider';
import {routePath} from '../../shell/routes';
import {Feedback} from '../../ui/Feedback';
import {Area,Field,Select} from '../../ui/Field';
import {lines} from '../../ui/text';
import {useAction} from '../../ui/useAction';
import {Drawer} from '../Drawer';
import {useNames} from './TaskDrawer';

// The complete task contract, for owners who already have a specification.
export function TaskCreateDrawer({projectId,onClose}:{projectId:string;onClose:()=>void}){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents(),[mode,setMode]=useState('integrator');
 const {names,people}=useNames(projectId);
 return <Drawer title={t('create.title')} onClose={onClose}>
  <p className="muted">{t('create.help')}</p>
  <form onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget);void action.run(async()=>{
   const r=await command<Schema['Task']>('task.create',{projectId,goal:String(data.get('goal')),scope:lines(data.get('scope')),nonScope:lines(data.get('nonScope')),authorityRefs:lines(data.get('authorityRefs')),risk:String(data.get('risk')),
    collaboratorLimit:Number(data.get('limit')),designatedIntegratorId:String(data.get('integrator')??'')||undefined,
    acceptancePolicy:{mode,acceptorId:mode==='designated-agent'?String(data.get('acceptor')):undefined,conditions:lines(data.get('conditions'))}});
   onClose();navigate(routePath({name:'room',projectId,drawer:{kind:'task',id:r.result.id,tab:'overview'}}));
  })}}>
   <fieldset disabled={!events.connected||action.busy}>
    <Field label={t('create.goal')} name="goal" required maxLength={8192}/>
    <Area label={t('create.scope')} name="scope" required/>
    <Area label={t('create.nonScope')} name="nonScope"/>
    <Area label={t('create.authority')} name="authorityRefs" required spellCheck={false}/>
    <Field label={t('composer.limit')} name="limit" type="number" min={0} max={64} defaultValue={0} required/>
    <Select label={t('create.risk')} name="risk" defaultValue="normal"><option value="normal">{t('risk.normal')}</option><option value="low">{t('risk.low')}</option><option value="high">{t('risk.high')}</option></Select>
    <Select label={t('composer.integrator')} name="integrator" defaultValue=""><option value="">{t('composer.anyIntegrator')}</option>{people.map(p=><option key={p.id} value={p.id}>{names[p.id]}</option>)}</Select>
    <Select label={t('create.acceptanceMode')} name="mode" value={mode} onChange={e=>setMode(e.target.value)}><option value="integrator">{t('acceptor.integrator')}</option><option value="human">{t('acceptor.human')}</option><option value="designated-agent">{t('acceptor.designated')}</option></Select>
    {mode==='designated-agent'?<Select label={t('create.acceptor')} name="acceptor" required><option value="">{t('form.choose')}</option>{people.map(p=><option key={p.id} value={p.id}>{names[p.id]}</option>)}</Select>:null}
    <Area label={t('create.conditions')} name="conditions" required/>
    <button type="submit">{t('create.submit')}</button>
   </fieldset>
   <Feedback error={action.error}/>
  </form>
 </Drawer>;
}
