import {useMemo,useState} from 'react';
import type {Schema} from '../../api/client';
import {useCollection} from '../../api/resource';
import {navigate} from '../../App';
import {useI18n} from '../../i18n/I18nProvider';
import {routePath} from '../../shell/routes';
import {Field,Select} from '../../ui/Field';
import {RelativeTime} from '../../ui/RelativeTime';
import {Drawer} from '../Drawer';

type Status='all'|'done'|'cancelled';
type Entry={key:string;text:string;status:'done'|'cancelled';at:string;path:string};
// Done and cancelled tasks plus cancelled requests, searchable by text.
export function ArchiveDrawer({projectId}:{projectId:string}){
 const {t}=useI18n(),[query,setQuery]=useState(''),[status,setStatus]=useState<Status>('all');
 const q=(kind:string)=>'/api/v1/queries/'+kind+'?projectId='+encodeURIComponent(projectId);
 const tasks=useCollection<Schema['Task']>(q('tasks')),requests=useCollection<Schema['WorkRequest']>(q('requests'));
 const entries=useMemo<Entry[]>(()=>[
  ...(tasks.data??[]).filter(x=>x.phase==='done'||x.phase==='cancelled').map(x=>({key:'t'+x.id,text:x.goal,status:x.phase as Entry['status'],at:x.completedAt??x.updatedAt??'',path:routePath({name:'room',projectId,drawer:{kind:'task',id:x.id,tab:'overview'}})})),
  ...(requests.data??[]).filter(x=>x.state==='cancelled').map(x=>({key:'r'+x.id,text:x.text,status:'cancelled' as const,at:x.createdAt,path:routePath({name:'room',projectId,drawer:{kind:'request',id:x.id}})})),
 ].sort((a,b)=>b.at.localeCompare(a.at)),[tasks.data,requests.data,projectId]);
 const needle=query.trim().toLowerCase();
 const shown=entries.filter(e=>(status==='all'||e.status===status)&&(!needle||e.text.toLowerCase().includes(needle)));
 return <Drawer title={t('archive.title')} onClose={()=>navigate(routePath({name:'room',projectId}))}>
  <Field label={t('archive.search')} type="search" value={query} onChange={e=>setQuery(e.target.value)}/>
  <Select label={t('archive.status')} value={status} onChange={e=>setStatus(e.target.value as Status)}>
   <option value="all">{t('archive.all')}</option><option value="done">{t('phase.done')}</option><option value="cancelled">{t('phase.cancelled')}</option>
  </Select>
  <ul className="record-list" aria-label={t('archive.results')}>
   {shown.map(e=><li key={e.key}><a href={e.path} data-local>{e.text}</a><p className="muted">{e.status==='done'?t('phase.done'):t('phase.cancelled')}{e.at?<> · <RelativeTime iso={e.at}/></>:null}</p></li>)}
  </ul>
  {!shown.length?<p className="muted">{t('archive.empty')}</p>:null}
 </Drawer>;
}
