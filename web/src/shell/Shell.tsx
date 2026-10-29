import {useMemo,type ReactNode} from 'react';
import '../theme/tokens.css';
import '../theme/base.css';
import './shell.css';
import type {Schema} from '../api/client';
import {useCollection} from '../api/resource';
import {useI18n} from '../i18n/I18nProvider';
import {IconGear,IconPlus} from '../ui/icons';
import {InkNav} from '../ui/InkNav';
import {useBump} from '../ui/motion';
import {Wordmark} from '../ui/Wordmark';
import {ConnectionBanner,ConnectionIndicator} from './ConnectionBanner';
import {NeedsYou} from './NeedsYou';
import {routePath,type Route} from './routes';
import {useNeedsYou} from './useNeedsYou';

type Project=Schema['ProjectView'];
// Rooms are ordered by creation, which is also the Needs-you grouping order.
export function useRooms(){
 const projects=useCollection<Project>('/api/v1/queries/projects');
 const rooms=useMemo(()=>[...(projects.data??[])].sort((a,b)=>(a.createdAt??'').localeCompare(b.createdAt??'')||a.id.localeCompare(b.id)),[projects.data]);
 return {rooms,loaded:projects.data!==undefined,error:projects.error,refresh:projects.refresh};
}

// A room tab; its owner count pops when the number changes.
function RoomTab({room,count,current}:{room:Project;count:number;current:boolean}){
 const {tn}=useI18n(),bump=useBump(count);
 return <a className="room-tab" href={'/rooms/'+room.id} data-local aria-current={current?'page':undefined}>
  <span className="room-name" title={room.name}>{room.name}</span>{count?<><span key={bump} className={'count'+(bump?' pop':'')} aria-hidden="true">{count}</span><span className="sr-only">, {tn('needsYou.items',count)}</span></>:null}
 </a>;
}

export function Shell({route,retry,busy,children}:{route:Route;retry:()=>void;busy:boolean;children:ReactNode}){
 const {t,tn}=useI18n(),{rooms}=useRooms();
 const order=useMemo(()=>rooms.map(r=>r.id),[rooms]);
 const needs=useNeedsYou(order);
 const active=route.name==='room'?route.projectId:undefined;
 return <div className="rm">
  <header className="shell-bar">
   <Wordmark/>
   <InkNav className="room-tabs" label={t('shell.rooms')} current={(active??'')+'|'+rooms.map(r=>r.name).join('|')}>
    {rooms.map(room=><RoomTab key={room.id} room={room} count={needs.byProject[room.id]?.length??0} current={room.id===active}/>)}
    <a className="room-tab room-new" href="/rooms/new" data-local title={t('shell.newRoom')}><IconPlus/><span className="sr-only">{t('shell.newRoom')}</span></a>
   </InkNav>
   <div className="topbar-end">
    <ConnectionIndicator/>
    <NeedsYou items={needs.items} rooms={rooms}/>
    <a className="gear" href={routePath({name:'settings'})} data-local title={t('shell.settings')}><IconGear/><span className="sr-only">{t('shell.settings')}</span></a>
   </div>
  </header>
  <ConnectionBanner retry={retry} busy={busy}/>
  <main id="main" className="room-main">{children}</main>
 </div>;
}
