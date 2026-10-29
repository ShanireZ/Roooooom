import {useI18n} from '../i18n/I18nProvider';
import {Stamp} from '../ui/Stamp';
import {routePath} from '../shell/routes';
import type {Note} from './floorModel';
import {useSeatLabels} from './Seat';

// Requests are notes at the door until an agent converts them into a table.
export function Door({notes,projectId}:{notes:Note[];projectId:string}){
 const {t}=useI18n(),labels=useSeatLabels();
 return <ul className="door" aria-label={t('room.door')}>
  {notes.map(note=><li key={note.requestId}>
   <a className="note" href={routePath({name:'room',projectId,drawer:{kind:'request',id:note.requestId}})} data-local title={note.text}>
    {note.flagged?<Stamp kind="formal">{t('flag.decision')}</Stamp>:null}
    <span className="note-text">{note.text}</span>
    <span className="muted">{note.state==='waiting'?t('request.waiting'):t('request.preparing')}</span>
    {note.preparer?<span className="muted">{labels.name(note.preparer)} · {labels.sync(note.preparer)}</span>:null}
   </a>
  </li>)}
 </ul>;
}
