import {useEffect,useRef,useState,type ReactNode} from 'react';
import './room.css';
import {useI18n} from '../i18n/I18nProvider';
import {Feedback} from '../ui/Feedback';
import {OpenDoorButton} from '../drawer/settings/RoomSettingsDrawer';
import {TaskCreateDrawer} from '../drawer/task/TaskCreateDrawer';
import {rememberRoom} from '../shell/home';
import {useRooms} from '../shell/Shell';
import {Activity} from './Activity';
import {Cabinet} from './Cabinet';
import {Composer,type ComposerHandle} from './Composer';
import {Door} from './DoorNote';
import {Floor} from './Floor';
import {Lobby} from './Lobby';
import {useRoomData} from './useRoomData';

// The room: door, floor, cabinet and lobby, with the activity column and the drawer beside it.
// Below 1200px a Floor | Activity switch shows one panel at a time.
export function RoomPage({projectId,name,drawer}:{projectId:string;name:string;drawer?:ReactNode}){
 const {t}=useI18n(),room=useRoomData(projectId),project=useRooms().rooms.find(r=>r.id===projectId);
 const [focusedMember,setFocusedMember]=useState<string>(),[view,setView]=useState<'floor'|'activity'>('floor'),[creating,setCreating]=useState(false);
 const composer=useRef<ComposerHandle>(null);
 useEffect(()=>rememberRoom(projectId),[projectId]);
 const select=(id:string)=>setFocusedMember(current=>current===id?undefined:id);
 const ask=()=>{setView('activity');composer.current?.ask()};
 const {floor}=room;
 const focused=focusedMember?[...floor.tables.flatMap(x=>x.seats),...floor.lobby].find(s=>s.participantId===focusedMember):undefined;
 return <section className="room" data-view={view}>
  <div className="view-switch segmented" role="group" aria-label={t('room.view')}>
   <button type="button" className="btn-ghost" aria-pressed={view==='floor'} onClick={()=>setView('floor')}>{t('room.floor')}</button>
   <button type="button" className="btn-ghost" aria-pressed={view==='activity'} onClick={()=>setView('activity')}>{t('activity.title')}</button>
  </div>
  <div className="room-floor">
   <header className="room-header">
    <h1 className="room-title">{name}</h1>
    {focusedMember?<button type="button" className="btn-ghost" onClick={()=>setFocusedMember(undefined)}>{t('seat.clearFocus')}{focused?' · '+(focused.name||t('seat.revoked')):''}</button>:null}
   </header>
   {project&&project.enrollmentEnabled===false?<div className="notice door-banner" role="status"><span>{t('door.banner')}</span> <OpenDoorButton project={project}/></div>:null}
   <Feedback error={room.error}/>
   <div className={'floor-plan'+(floor.density==='compact'?' compact':'')}>
    <div className="door-col"><h2 className="label">{t('room.door')}</h2><Door notes={floor.notes} projectId={projectId}/></div>
    <div className="floor-col">
     <h2 className="label">{t('room.floor')}</h2>
     {room.loaded&&!floor.tables.length&&!floor.notes.length?<p className="muted empty-floor">{t('room.emptyFloor')}</p>:null}
     <Floor tables={floor.tables} projectId={projectId} compact={floor.density==='compact'} focusedMember={focusedMember} onSelectMember={select} onAsk={ask}/>
    </div>
    <div className="cabinet-col"><Cabinet cabinet={floor.cabinet} projectId={projectId}/></div>
   </div>
   <Lobby members={floor.lobby} anyMembers={room.participants.length>0} projectId={projectId} focusedMember={focusedMember} onSelectMember={select}/>
  </div>
  <aside className="room-activity">
   <Activity projectId={projectId}/>
   <Composer ref={composer} projectId={projectId} participants={room.participants} onCreateTask={()=>setCreating(true)}/>
  </aside>
  {creating?<TaskCreateDrawer projectId={projectId} onClose={()=>setCreating(false)}/>:drawer}
 </section>;
}
