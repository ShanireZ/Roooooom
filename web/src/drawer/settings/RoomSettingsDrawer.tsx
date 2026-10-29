import {useState} from 'react';
import {command,type Schema} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {useCollection} from '../../api/resource';
import {navigate} from '../../App';
import {useI18n} from '../../i18n/I18nProvider';
import {JoinInstructions} from '../../setup/JoinInstructions';
import {routePath,type SettingsSection} from '../../shell/routes';
import {useRooms} from '../../shell/Shell';
import {Feedback} from '../../ui/Feedback';
import {InkNav} from '../../ui/InkNav';
import {RelativeTime} from '../../ui/RelativeTime';
import {useAction} from '../../ui/useAction';
import {Drawer} from '../Drawer';
import {useNames} from '../task/TaskDrawer';
import {IntegrationsSection} from './IntegrationsSection';

const SECTIONS:SettingsSection[]=['door','members','integrations'];
export function RoomSettingsDrawer({projectId,section}:{projectId:string;section:SettingsSection}){
 const {t}=useI18n(),{rooms}=useRooms(),project=rooms.find(r=>r.id===projectId);
 return <Drawer title={t('settings.title')} onClose={()=>navigate(routePath({name:'room',projectId}))}>
  <InkNav className="drawer-tabs" label={t('settings.sections')} current={section}>
   {SECTIONS.map(s=><a key={s} href={routePath({name:'room',projectId,drawer:{kind:'settings',section:s}})} data-local aria-current={s===section?'page':undefined}>{t(('settings.'+s) as 'settings.door')}</a>)}
  </InkNav>
  {!project?null:section==='door'?<DoorSection project={project}/>:section==='members'?<MembersSection projectId={projectId}/>:<IntegrationsSection projectId={projectId}/>}
 </Drawer>;
}

export function OpenDoorButton({project}:{project:Schema['ProjectView']}){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents(),{refresh}=useRooms();
 return <><button type="button" disabled={!events.connected||action.busy} onClick={()=>void action.run(async()=>{await command('enrollment.enable',{},project.id);refresh()})}>{t('door.open')}</button><Feedback error={action.error}/></>;
}

function DoorSection({project}:{project:Schema['ProjectView']}){
 const {t}=useI18n();
 return <>
  <h3>{t('door.repos')}</h3>
  <ul className="plain-list">{project.repositories.map(r=><li key={r.id} className="mono">{r.name} · {r.reference}</li>)}</ul>
  {project.enrollmentEnabled?<JoinInstructions project={project}/>:<><p className="notice">{t('door.closed')}</p><p className="muted">{t('door.consent',{name:project.name})}</p><OpenDoorButton project={project}/></>}
 </>;
}

function MembersSection({projectId}:{projectId:string}){
 const {t}=useI18n(),action=useAction(),events=useWorkspaceEvents(),{names,people}=useNames(projectId);
 const sessions=useCollection<Schema['SessionView']>('/api/v1/queries/sessions?projectId='+encodeURIComponent(projectId));
 const [path,setPath]=useState(''),[revoking,setRevoking]=useState('');
 return <>
  {!people.length?<p className="muted">{t('members.empty')}</p>:null}
  <ul className="record-list" aria-label={t('settings.members')}>
   {people.map(p=><li key={p.id}>
    <p><strong>{names[p.id]}</strong> <span className="mono muted">{p.id}</span></p>
    <ul className="plain-list" aria-label={t('members.sessions')}>{sessions.data?.filter(s=>s.participantId===p.id).map(s=><li key={s.id}>{t(('session.status.'+s.status) as 'session.status.active')} · {t('shell.lastSync')} <RelativeTime iso={s.lastSeenAt}/> <span className="mono">{s.id.slice(0,8)}</span></li>)}</ul>
    <div className="actions">
     <button type="button" className="btn-ghost" disabled={!events.connected||action.busy} onClick={()=>void action.run(async()=>{const r=await command<{credentialPath:string}>('participant.recover',{},p.id);setPath(r.result.credentialPath);action.setNotice(t('members.recovered'))})}>{t('members.recover')}</button>
     {revoking===p.id?<><span className="notice">{t('members.revokeConfirm')}</span><button type="button" className="btn-danger" disabled={action.busy} onClick={()=>void action.run(async()=>{await command('participant.revoke',{},p.id);setRevoking('');action.setNotice(t('members.revoked'))})}>{t('members.confirmRevoke')}</button><button type="button" className="btn-ghost" onClick={()=>setRevoking('')}>{t('form.close')}</button></>
      :<button type="button" className="btn-danger" disabled={!events.connected||action.busy} onClick={()=>setRevoking(p.id)}>{t('members.revoke')}</button>}
    </div>
   </li>)}
  </ul>
  {path?<p className="mono">{path}<br/>{t('members.privatePath')}</p>:null}
  <Feedback error={action.error??sessions.error} notice={action.notice}/>
 </>;
}
