import {useCallback,useEffect,useLayoutEffect,useMemo,useRef,useState} from 'react';
import {apiRequest,type Schema} from '../api/client';
import {useWorkspaceEvents} from '../api/events';
import {useCollection,useResource} from '../api/resource';
import {MessageRow} from '../drawer/task/DiscussionTab';
import {useI18n} from '../i18n/I18nProvider';
import {hasMessage} from '../i18n/translator';
import {routePath} from '../shell/routes';
import {RelativeTime} from '../ui/RelativeTime';
import {Stamp} from '../ui/Stamp';

// Project-thread messages come from the thread read; task-thread ones are read by id
// and link back to their task's discussion.
function FeedChat({entry,known,names,projectId,goals}:{entry:Schema['ActivityEntry'];known?:Schema['Message'];names:Record<string,string>;projectId:string;goals:Map<string,string>}){
 const {t}=useI18n(),fetched=useResource<Schema['Message']>('/api/v1/queries/message?targetId='+encodeURIComponent(entry.resourceId),!known);
 const message=known??fetched.data,taskId=message?.taskId??entry.taskId,goal=taskId?goals.get(taskId):undefined;
 const context=taskId&&goal?<a className="entry-context" href={routePath({name:'room',projectId,drawer:{kind:'task',id:taskId,tab:'discussion'}})} data-local>{goal}</a>:null;
 if(!message)return <li className="entry entry-chat"><div className="message-meta"><span className="chat-label">{t('activity.chat')}</span><RelativeTime iso={entry.receivedAt}/></div></li>;
 return <MessageRow message={message} names={names} className="entry entry-chat" context={context}/>;
}

type Entry=Schema['ActivityEntry'];
type Message=Schema['Message'];
type Filter='all'|'chat'|'progress'|'decisions';
const category=(kind:string):Exclude<Filter,'all'>=>kind==='message.send'?'chat':kind.startsWith('decision.')?'decisions':'progress';

// Newest-first pages from the server; older pages load on demand.
function useActivity(projectId:string){
 const events=useWorkspaceEvents(),[latest,setLatest]=useState<Entry[]>([]),[older,setOlder]=useState<Entry[]>([]);
 const [cursor,setCursor]=useState(''),[error,setError]=useState('');
 const base='/api/v1/queries/activity?projectId='+encodeURIComponent(projectId)+'&limit=100';
 useEffect(()=>{
  if(!events.connected)return;
  let current=true;
  apiRequest<Entry[]>(base).then(r=>{if(current){setLatest(r.result);if(!older.length)setCursor(r.hasMore?r.cursor:'')}}).catch(()=>{if(current)setError('offline')});
  return()=>{current=false};
  // eslint-disable-next-line react-hooks/exhaustive-deps
 },[base,events.connected,events.revision]);
 const loadOlder=useCallback(()=>{
  if(!cursor)return;
  void apiRequest<Entry[]>(base+'&cursor='+encodeURIComponent(cursor)).then(r=>{setOlder(o=>[...o,...r.result]);setCursor(r.hasMore?r.cursor:'')}).catch(()=>setError('offline'));
 },[base,cursor]);
 const entries=useMemo(()=>{const seen=new Set<number>();return [...latest,...older].filter(e=>!seen.has(e.seq)&&seen.add(e.seq))},[latest,older]);
 return {entries,loadOlder,canLoadOlder:!!cursor,error};
}

export function Activity({projectId}:{projectId:string}){
 const {t}=useI18n(),[filter,setFilter]=useState<Filter>('all');
 const {entries,loadOlder,canLoadOlder}=useActivity(projectId);
 const q=(kind:string)=>'/api/v1/queries/'+kind+'?projectId='+encodeURIComponent(projectId);
 const messages=useCollection<Message>(q('thread')),tasks=useCollection<Schema['Task']>(q('tasks'));
 const requests=useCollection<Schema['WorkRequest']>(q('requests')),people=useCollection<Schema['ParticipantView']>(q('participants'));
 const byId=useMemo(()=>({message:new Map((messages.data??[]).map(x=>[x.id,x])),task:new Map((tasks.data??[]).map(x=>[x.id,x.goal])),
  request:new Map((requests.data??[]).map(x=>[x.id,x.text])),person:new Map((people.data??[]).map(x=>[x.id,x.name]))}),[messages.data,tasks.data,requests.data,people.data]);
 const names=useMemo(()=>Object.fromEntries(byId.person),[byId.person]);
 const actor=(e:Entry)=>e.actorKind==='human'?t('activity.you'):e.actorKind==='system'?t('activity.system'):byId.person.get(e.actorId??'')??t('seat.revoked');
 const shown=entries.filter(e=>filter==='all'||category(e.kind)===filter).slice().reverse();
 // Follow the newest entry, but keep the reader in place when older entries are prepended.
 const list=useRef<HTMLOListElement>(null),fromBottom=useRef<number|null>(null),newest=entries[0]?.seq;
 useEffect(()=>{const el=list.current;if(el)el.scrollTop=el.scrollHeight},[newest,filter]);
 useLayoutEffect(()=>{const el=list.current;if(el&&fromBottom.current!==null){el.scrollTop=el.scrollHeight-fromBottom.current;fromBottom.current=null}},[shown.length]);
 const older=()=>{const el=list.current;if(el)fromBottom.current=el.scrollHeight-el.scrollTop;loadOlder()};
 return <section className="activity" aria-label={t('activity.title')}>
  <div className="chips" role="group" aria-label={t('activity.filters')}>
   {(['all','chat','progress','decisions'] as Filter[]).map(f=><button key={f} type="button" className="chip" aria-pressed={filter===f} onClick={()=>setFilter(f)}>{t(('activity.filter.'+f) as 'activity.filter.all')}</button>)}
  </div>
  <ol ref={list} className="feed" aria-label={t('activity.feed')}>
   {canLoadOlder?<li className="feed-older"><button type="button" className="btn-text" onClick={older}>{t('activity.older')}</button></li>:null}
   {shown.map(e=>{
    const subject=byId.task.get(e.taskId??'')??byId.request.get(e.requestId??'')??byId.request.get(e.resourceId)??'';
    const when=<span className="muted"><RelativeTime iso={e.receivedAt}/></span>;
    if(e.kind==='message.send')return <FeedChat key={e.seq} entry={e} known={byId.message.get(e.resourceId)} names={names} projectId={projectId} goals={byId.task}/>;
    const key='activity.kind.'+e.kind,text=hasMessage(key)?t(key,{actor:actor(e),subject}):t('activity.generic',{actor:actor(e),kind:e.kind});
    const formal=e.kind.startsWith('decision.');
    return <li key={e.seq} className={'entry'+(formal?' entry-formal':'')}>
     {formal?<Stamp kind="formal"/>:null}
     {formal&&e.kind==='decision.request'?<a href={routePath({name:'room',projectId,drawer:{kind:'decision',id:e.resourceId}})} data-local>{text}</a>:<span>{text}</span>} {when}
    </li>;
   })}
  </ol>
 </section>;
}
