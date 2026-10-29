import {useState} from 'react';
import type {Schema} from '../../api/client';
import {useCollection,useResource} from '../../api/resource';
import {useI18n} from '../../i18n/I18nProvider';
import {ChatForm} from '../../room/Composer';
import {Feedback} from '../../ui/Feedback';
import {RelativeTime} from '../../ui/RelativeTime';
import {Stamp} from '../../ui/Stamp';
import type {TaskView} from './TaskDrawer';

type Message=Schema['Message'];
// The task thread; text is rendered as text, never as Markdown or HTML.
export function DiscussionTab({task,names}:{task:TaskView;names:Record<string,string>}){
 const {t}=useI18n();
 const messages=useCollection<Message>('/api/v1/queries/thread?targetId='+encodeURIComponent(task.id));
 return <>
  <ul className="record-list" aria-label={t('task.tab.discussion')}>
   {messages.data?.map(m=><MessageRow key={m.id} message={m} names={names}/>)}
  </ul>
  {!messages.data?.length?<p className="muted">{t('discussion.empty')}</p>:null}
  <ChatForm projectId={task.projectId} task={task}/>
 </>;
}
function MessageRow({message,names}:{message:Message;names:Record<string,string>}){
 const {t}=useI18n(),[open,setOpen]=useState(false);
 const detail=useResource<Message&{receipts?:{participantId:string;state:string;version:number}[]}>('/api/v1/queries/message?targetId='+encodeURIComponent(message.id),open);
 return <li>
  <div className="message-meta"><strong>{message.actorKind==='human'?t('activity.you'):names[message.actorId]??t('seat.revoked')}</strong><span className="chat-label">{t('activity.chat')}</span><RelativeTime iso={message.receivedAt}/></div>
  {message.imported?<p><Stamp kind="offline"/> <span className="muted">{t('activity.importedAt')} <RelativeTime iso={message.localOccurredAt??''}/></span></p>:null}
  <pre>{message.body}</pre>
  <p className="muted">{t(('purpose.'+message.purpose) as 'purpose.discussion')}{message.actionRequired?' · '+t('discussion.actionRequired'):''}</p>
  <details className="fold" onToggle={e=>setOpen(e.currentTarget.open)}><summary>{t('discussion.receipts')}</summary>
   <ul className="plain-list">{detail.data?.receipts?.map(r=><li key={r.participantId}>{names[r.participantId]??r.participantId} · {r.state} · v{r.version}</li>)}</ul>
   <Feedback error={detail.error}/>
  </details>
 </li>;
}
