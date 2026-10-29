import {useI18n} from '../i18n/I18nProvider';
import {RoomPage} from '../room/RoomPage';
import type {Route} from './routes';
import {useRooms} from './Shell';

// Route content inside the shell; later tasks replace each branch with its page.
export function RoutePage({route}:{route:Route}){
 const {t}=useI18n(),{rooms,loaded}=useRooms();
 if(route.name==='room'){
  const room=rooms.find(r=>r.id===route.projectId);
  if(!room)return loaded?<p className="page-message">{t('room.notFound')}</p>:null;
  return <RoomPage key={room.id} projectId={room.id} name={room.name} route={route.drawer}/>;
 }
 return <p className="page-message">{t('route.notFound')}</p>;
}
