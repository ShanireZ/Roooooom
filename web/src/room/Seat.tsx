import {useI18n} from '../i18n/I18nProvider';
import {formatDateTime,formatRelative} from '../i18n/format';
import {useNow} from '../ui/RelativeTime';
import type {Seat as SeatModel} from './floorModel';

const initials=(name:string)=>{
 const words=name.trim().split(/\s+/).filter(Boolean);
 if(!words.length)return '?';
 return /^[A-Za-z0-9]/.test(words[0])?(words.length>1?words[0][0]+words[1][0]:words[0].slice(0,2)).toUpperCase():[...words[0]][0];
};
export function useSeatLabels(){
 const {t,locale}=useI18n(),now=useNow();
 const name=(s:SeatModel)=>(s.revoked?t('seat.revoked'):s.name)+(s.showShortId?' · '+s.shortId:'');
 const role=(s:SeatModel)=>s.role==='integrating'?t('seat.role.integrating'):s.role==='delivered'?t('seat.role.delivered'):s.role==='preparing'?t('seat.role.preparing'):s.role;
 // Only an observation: never "online" or "working".
 const sync=(s:SeatModel)=>s.ended?t('seat.ended'):s.lastSeenAt&&!s.lastSeenAt.startsWith('0001-')?t('time.lastSynced',{time:formatRelative(s.lastSeenAt,now,locale)}):t('time.never');
 const title=(s:SeatModel)=>s.lastSeenAt&&!s.lastSeenAt.startsWith('0001-')?formatDateTime(s.lastSeenAt,locale):undefined;
 return {name,role,sync,title};
}
// A seat is a button: selecting it focuses the room on that member's work.
export function Seat({seat,pressed,onSelect,compact=false,entering=false}:{seat:SeatModel;pressed:boolean;onSelect:(participantId:string)=>void;compact?:boolean;entering?:boolean}){
 const labels=useSeatLabels();
 return <button type="button" className={'seat'+(entering?' join':'')} data-stale={String(seat.stale||seat.ended)} aria-pressed={pressed} title={labels.title(seat)} onClick={()=>onSelect(seat.participantId)}>
  <span className="avatar" style={{background:`var(--agent-${seat.color})`}} aria-hidden="true">{seat.revoked?'?':initials(seat.name)}</span>
  <span className="seat-name" title={labels.name(seat)}>{labels.name(seat)}</span>
  {seat.role?<span className="seat-role">{labels.role(seat)}</span>:null}
  {compact?null:<span className="seat-sync">{labels.sync(seat)}</span>}
 </button>;
}
