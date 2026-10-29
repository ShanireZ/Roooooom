import type {Schema} from '../../api/client';
import {useCollection} from '../../api/resource';
import {useI18n} from '../../i18n/I18nProvider';
import {routePath} from '../../shell/routes';
import {RelativeTime} from '../../ui/RelativeTime';
import {Stamp} from '../../ui/Stamp';
import type {TaskView} from './TaskDrawer';

// Formal decisions about this task, newest version state first.
export function HistoryTab({task}:{task:TaskView}){
 const {t}=useI18n();
 const decisions=useCollection<Schema['DecisionRequestView']>('/api/v1/queries/decisions?targetId='+encodeURIComponent(task.id));
 if(!decisions.data?.length)return <p className="muted">{t('history.empty')}</p>;
 return <ul className="record-list" aria-label={t('task.tab.history')}>
  {decisions.data.map(d=><li key={d.id}>
   <Stamp kind="formal"/> <a href={routePath({name:'room',projectId:task.projectId,drawer:{kind:'decision',id:d.id}})} data-local>{d.question}</a>
   <p className="muted">{t(('decision.state.'+d.state) as 'decision.state.pending')} · v{d.version}{d.resolutionOutcome?' · '+d.resolutionOutcome:''} · <RelativeTime iso={d.receivedAt}/></p>
  </li>)}
 </ul>;
}
