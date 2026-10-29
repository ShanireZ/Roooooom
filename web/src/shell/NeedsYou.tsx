import {useEffect,useRef,useState} from 'react';
import {localizedSummary} from '../drawer/decision/systemQuestions';
import {blockingKey} from '../drawer/task/blocking';
import {useI18n} from '../i18n/I18nProvider';
import {useBump} from '../ui/motion';
import {Stamp} from '../ui/Stamp';
import {RelativeTime} from '../ui/RelativeTime';
import {needsYouKey,needsYouPath,type NeedsYouItem} from './useNeedsYou';

type Room={id:string;name:string};
// A blocker says why it blocks; the summary already names the table or request.
const reason=(item:NeedsYouItem)=>item.kind==='blocker'&&item.code?blockingKey(item.code):undefined;
// A popover list grouped by room; choosing an item opens its drawer.
export function NeedsYou({items,rooms}:{items:NeedsYouItem[];rooms:Room[]}){
 const {t,tn}=useI18n(),panel=useRef<HTMLDivElement>(null),[open,setOpen]=useState(false);
 const [announced,setAnnounced]=useState(''),lastAnnounce=useRef(0);
 useEffect(()=>{
  const el=panel.current;if(!el)return;
  const toggle=(e:Event)=>setOpen((e as ToggleEvent).newState==='open');
  el.addEventListener('toggle',toggle);return()=>el.removeEventListener('toggle',toggle);
 },[]);
 // Count changes are announced politely, at most once per 10 seconds.
 useEffect(()=>{
  const wait=Math.max(0,10000-(Date.now()-lastAnnounce.current));
  const timer=setTimeout(()=>{lastAnnounce.current=Date.now();setAnnounced(tn('needsYou.announce',items.length))},wait);
  return()=>clearTimeout(timer);
 },[items.length,tn]);
 const bump=useBump(items.length);
 const groups=rooms.filter(r=>items.some(i=>i.projectId===r.id));
 return <>
  <button type="button" className="needs-button" popoverTarget="needs-you-panel" aria-expanded={open} aria-controls="needs-you-panel">
   {t('shell.needsYou')}{items.length?<span key={bump} className={'count'+(bump?' pop':'')} aria-hidden="true">{items.length}</span>:null}<span className="sr-only">{tn('needsYou.items',items.length)}</span>
  </button>
  <span className="sr-only" aria-live="polite">{announced}</span>
  <div id="needs-you-panel" popover="auto" ref={panel} className="needs-panel">
   <p className="label">{t('shell.needsYou')} · {items.length}</p>
   {items.length?<ul aria-label={t('shell.needsYou')} className="needs-groups">{groups.map(room=><li key={room.id}>
    <h3 className="needs-room">{room.name}</h3>
    <ul className="needs-items">{items.filter(i=>i.projectId===room.id).map(item=><li key={needsYouKey(item)}>
     <a href={needsYouPath(item)} data-local onClick={()=>panel.current?.hidePopover()}>
      <Stamp kind={item.kind==='decision'?'formal':item.kind}/> <span className="needs-summary">{localizedSummary(item,t)}</span>
      {reason(item)?<span className="muted needs-reason">{t(reason(item)!)}</span>:null}
      <span className="muted needs-since"><RelativeTime iso={item.since}/></span>
     </a>
    </li>)}</ul>
   </li>)}</ul>:<p className="muted">{t('needsYou.empty')}</p>}
  </div>
 </>;
}
