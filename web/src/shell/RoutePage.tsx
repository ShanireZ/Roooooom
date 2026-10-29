import {useI18n} from '../i18n/I18nProvider';
import type {Route} from './routes';
import {useRooms} from './Shell';

// Route content inside the shell; later tasks replace each branch with its page.
export function RoutePage({route}:{route:Route}){
 const {t}=useI18n(),{rooms,loaded}=useRooms();
 if(route.name==='room'){
  const room=rooms.find(r=>r.id===route.projectId);
  if(!room)return loaded?<p className="page-message">{t('room.notFound')}</p>:null;
  return <section className="room"><h1 className="room-title">{room.name}</h1></section>;
 }
 return <p className="page-message">{t('route.notFound')}</p>;
}
