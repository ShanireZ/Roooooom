import type {Schema} from '../../api/client';
import {useCollection,useResource} from '../../api/resource';
import {navigate} from '../../App';
import {useI18n} from '../../i18n/I18nProvider';
import {routePath,type TaskTab} from '../../shell/routes';
import {Feedback} from '../../ui/Feedback';
import {Drawer} from '../Drawer';
import {DiscussionTab} from './DiscussionTab';
import {EvidenceTab} from './EvidenceTab';
import {HistoryTab} from './HistoryTab';
import {ManageTab} from './ManageTab';
import {WorkTab} from './WorkTab';
import {OverviewTab} from './OverviewTab';
import {nextStep} from './nextStep';

export type TaskView=Schema['Task'];
const TABS:TaskTab[]=['overview','work','evidence','discussion','history','manage'];

export function useNames(projectId:string){
 const people=useCollection<Schema['ParticipantView']>('/api/v1/queries/participants?projectId='+encodeURIComponent(projectId));
 const list=people.data??[];
 const names:Record<string,string>={};
 for(const p of list)names[p.id]=p.name+(list.filter(x=>x.name===p.name).length>1?' · '+p.id.slice(0,8):'');
 return {names,people:list,fresh:people.fresh};
}

export function TaskDrawer({projectId,taskId,tab}:{projectId:string;taskId:string;tab:TaskTab}){
 const {t}=useI18n();
 const task=useResource<TaskView>('/api/v1/queries/task?targetId='+encodeURIComponent(taskId));
 const assignments=useCollection<Schema['Assignment']>('/api/v1/queries/assignments?targetId='+encodeURIComponent(taskId));
 const needs=useCollection<Schema['NeedsYouItem']>('/api/v1/queries/needs-you');
 const {names,people}=useNames(projectId);
 const close=()=>navigate(routePath({name:'room',projectId}));
 if(!task.data)return <Drawer title={task.error?t('task.notFound'):t('task.loading')} onClose={close}><Feedback error={task.error}/></Drawer>;
 const tk=task.data,step=nextStep({task:tk,assignments:assignments.data??[],needsYou:needs.data??[],names});
 const shown=TABS.includes(tab)?tab:'overview';
 return <Drawer title={tk.goal} onClose={close}>
  <div className="drawer-summary">
   <span className="label">{t(('phase.'+tk.phase) as 'phase.active')}</span>
   {tk.integratorId?<span className="muted">{t('task.responsible')}: {names[tk.integratorId]??t('seat.revoked')}</span>:null}
   <p className="next-step">{t(step.key,step.params)}</p>
  </div>
  <nav className="drawer-tabs" aria-label={t('task.tabs')}>
   {TABS.map(x=><a key={x} href={routePath({name:'room',projectId,drawer:{kind:'task',id:taskId,tab:x}})} data-local aria-current={x===shown?'page':undefined}>{t(('task.tab.'+x) as 'task.tab.overview')}</a>)}
  </nav>
  <Feedback error={task.error}/>
  {shown==='overview'?<OverviewTab task={tk}/>:shown==='work'?<WorkTab task={tk} names={names} fresh={task.fresh}/>:shown==='manage'?<ManageTab task={tk} names={names} people={people} fresh={task.fresh}/>:shown==='evidence'?<EvidenceTab task={tk} names={names}/>:shown==='discussion'?<DiscussionTab task={tk} names={names}/>:<HistoryTab task={tk}/>}
 </Drawer>;
}
