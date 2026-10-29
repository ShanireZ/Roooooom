import {useLayoutEffect,useRef} from 'react';
import {useI18n} from '../i18n/I18nProvider';
import {leaveGhost} from '../ui/motion';
import {Stamp} from '../ui/Stamp';
import {routePath} from '../shell/routes';
import type {Note} from './floorModel';
import {useSeatLabels} from './Seat';

// Requests are notes at the door until an agent converts them into a table.
export function Door({notes,projectId}:{notes:Note[];projectId:string}){
 const {t}=useI18n(),labels=useSeatLabels(),list=useRef<HTMLUListElement>(null),boxes=useRef(new Map<string,DOMRect>());
 // A note that leaves the door leaves its last box behind, so its table can grow out of it.
 useLayoutEffect(()=>{
  const next=new Map<string,DOMRect>();
  list.current?.querySelectorAll<HTMLElement>('[data-request-id]').forEach(el=>next.set(el.dataset.requestId!,el.getBoundingClientRect()));
  for(const [id,box] of boxes.current)if(!next.has(id))leaveGhost(id,box);
  boxes.current=next;
 });
 return <ul ref={list} className="door" aria-label={t('room.door')}>
  {notes.map(note=><li key={note.requestId} data-request-id={note.requestId}>
   <a className="note" href={routePath({name:'room',projectId,drawer:{kind:'request',id:note.requestId}})} data-local title={note.text}>
    {note.flagged?<Stamp kind="formal">{t('flag.decision')}</Stamp>:null}
    <span className="note-text">{note.text}</span>
    <span className="muted">{note.state==='waiting'?t('request.waiting'):t('request.preparing')}</span>
    {note.preparer?<span className="muted">{labels.name(note.preparer)} · {labels.sync(note.preparer)}</span>:null}
   </a>
  </li>)}
 </ul>;
}
