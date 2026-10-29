import {useI18n} from '../i18n/I18nProvider';
import {AcceptanceDrawer} from '../drawer/acceptance/AcceptanceDrawer';
import {ArchiveDrawer} from '../drawer/archive/ArchiveDrawer';
import {DecisionDrawer} from '../drawer/decision/DecisionDrawer';
import {RequestDrawer} from '../drawer/request/RequestDrawer';
import {TaskDrawer} from '../drawer/task/TaskDrawer';
import {RoomSettingsDrawer} from '../drawer/settings/RoomSettingsDrawer';
import {RoomPage} from '../room/RoomPage';
import {NewRoom} from '../setup/NewRoom';
import {SystemSettings} from '../settings/SystemSettings';
import type {DrawerRoute,Route} from './routes';
import {useRooms} from './Shell';

// The drawer for a room route; later tasks add the remaining drawer kinds.
function drawerFor(projectId:string,drawer?:DrawerRoute){
 if(drawer?.kind==='task')return <TaskDrawer key={'task-'+drawer.id} projectId={projectId} taskId={drawer.id} tab={drawer.tab}/>;
 if(drawer?.kind==='request')return <RequestDrawer key={'request-'+drawer.id} projectId={projectId} requestId={drawer.id}/>;
 if(drawer?.kind==='decision')return <DecisionDrawer key={'decision-'+drawer.id} projectId={projectId} decisionId={drawer.id}/>;
 if(drawer?.kind==='acceptance')return <AcceptanceDrawer key={'acceptance-'+drawer.taskId} projectId={projectId} taskId={drawer.taskId}/>;
 if(drawer?.kind==='archive')return <ArchiveDrawer key="archive" projectId={projectId}/>;
 if(drawer?.kind==='settings')return <RoomSettingsDrawer key="settings" projectId={projectId} section={drawer.section}/>;
 return undefined;
}

export function RoutePage({route}:{route:Route}){
 const {t}=useI18n(),{rooms,loaded}=useRooms();
 if(route.name==='newRoom')return <NewRoom/>;
 if(route.name==='settings')return <SystemSettings/>;
 if(route.name==='room'){
  const room=rooms.find(r=>r.id===route.projectId);
  if(!room)return loaded?<p className="page-message">{t('room.notFound')}</p>:null;
  return <RoomPage key={room.id} projectId={room.id} name={room.name} drawer={drawerFor(room.id,route.drawer)}/>;
 }
 return <p className="page-message">{t('route.notFound')}</p>;
}
