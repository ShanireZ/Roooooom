import {useEffect,useMemo} from 'react';
import {useResource} from '../api/resource';
import {navigate} from '../App';
import {useI18n} from '../i18n/I18nProvider';
import {homeTarget,readLastRoom} from './home';
import {routePath} from './routes';
import {useRooms} from './Shell';
import {useNeedsYou} from './useNeedsYou';

// `/`, `/projects` and `/decisions` resolve once rooms and Needs you have loaded;
// every redirect replaces the history entry so Back never bounces.
export function Home({openNeedsYou=false}:{openNeedsYou?:boolean}){
 const {rooms,loaded,error}=useRooms(),order=useMemo(()=>rooms.map(r=>r.id),[rooms]),needs=useNeedsYou(order);
 const ready=(loaded||!!error)&&(needs.loaded||!!needs.error);
 useEffect(()=>{
  if(!ready)return;
  navigate(homeTarget(rooms,needs.items,readLastRoom()),true);
  if(openNeedsYou)requestAnimationFrame(()=>{const panel=document.getElementById('needs-you-panel');if(panel&&!panel.matches(':popover-open'))panel.showPopover()});
 },[ready,rooms,needs.items,openNeedsYou]);
 return null;
}

export function Replace({to}:{to:string}){
 useEffect(()=>navigate(to,true),[to]);
 return null;
}

// `/tasks/:id` and `/requests/:id` read the record to find its room.
export function LegacyObject({kind,id}:{kind:'task'|'request';id:string}){
 const {t}=useI18n();
 const record=useResource<{projectId:string}>('/api/v1/queries/'+kind+'?targetId='+encodeURIComponent(id));
 const projectId=record.data?.projectId;
 useEffect(()=>{
  if(projectId)navigate(routePath({name:'room',projectId,drawer:kind==='task'?{kind:'task',id,tab:'overview'}:{kind:'request',id}}),true);
 },[projectId,kind,id]);
 return record.error?<p className="page-message">{t(kind==='task'?'task.notFound':'request.notFound')}</p>:null;
}
