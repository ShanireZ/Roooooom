import {useState} from 'react';
import './room.css';
import {useI18n} from '../i18n/I18nProvider';
import {Feedback} from '../ui/Feedback';
import type {DrawerRoute} from '../shell/routes';
import {Cabinet} from './Cabinet';
import {Door} from './DoorNote';
import {Floor} from './Floor';
import {Lobby} from './Lobby';
import {useRoomData} from './useRoomData';

// The room: door, floor, cabinet and lobby, with the activity column and drawer beside it.
export function RoomPage({projectId,name,onAsk=()=>{},activity,drawer}:{projectId:string;name:string;onAsk?:()=>void;activity?:React.ReactNode;drawer?:React.ReactNode;route?:DrawerRoute}){
 const {t}=useI18n(),room=useRoomData(projectId);
 const [focusedMember,setFocusedMember]=useState<string>();
 const select=(id:string)=>setFocusedMember(current=>current===id?undefined:id);
 const {floor}=room;
 const focusedName=focusedMember?[...floor.tables.flatMap(x=>x.seats),...floor.lobby].find(s=>s.participantId===focusedMember):undefined;
 return <section className="room">
  <div className="room-floor">
   <header className="room-header">
    <h1 className="room-title">{name}</h1>
    {focusedMember?<button type="button" className="btn-ghost" onClick={()=>setFocusedMember(undefined)}>{t('seat.clearFocus')}{focusedName?' · '+(focusedName.name||t('seat.revoked')):''}</button>:null}
   </header>
   <Feedback error={room.error}/>
   <div className={'floor-plan'+(floor.density==='compact'?' compact':'')}>
    <div className="door-col"><h2 className="label">{t('room.door')}</h2><Door notes={floor.notes} projectId={projectId}/></div>
    <div className="floor-col">
     <h2 className="label">{t('room.floor')}</h2>
     {room.loaded&&!floor.tables.length&&!floor.notes.length?<p className="muted empty-floor">{t('room.emptyFloor')}</p>:null}
     <Floor tables={floor.tables} projectId={projectId} compact={floor.density==='compact'} focusedMember={focusedMember} onSelectMember={select} onAsk={onAsk}/>
    </div>
    <div className="cabinet-col"><Cabinet cabinet={floor.cabinet} projectId={projectId}/></div>
   </div>
   <Lobby members={floor.lobby} anyMembers={room.participants.length>0} projectId={projectId} focusedMember={focusedMember} onSelectMember={select}/>
  </div>
  {activity?<aside className="room-activity">{activity}</aside>:null}
  {drawer}
 </section>;
}
