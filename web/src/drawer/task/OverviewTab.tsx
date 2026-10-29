import type {Schema} from '../../api/client';
import {useResource} from '../../api/resource';
import {useI18n} from '../../i18n/I18nProvider';
import {TechnicalDetails} from '../../ui/Fold';
import type {TaskView} from './TaskDrawer';
import {blockingKey} from './blocking';

export function List({items}:{items?:string[]}){
 const {t}=useI18n();
 return items?.length?<ul className="plain-list">{items.map((x,i)=><li key={i}>{x}</li>)}</ul>:<p className="muted">{t('common.none')}</p>;
}

// Plain language on the surface; identifiers and versions one layer down.
export function OverviewTab({task}:{task:TaskView}){
 const {t}=useI18n();
 const candidate=useResource<Schema['CandidateStatusView']>('/api/v1/queries/candidate?targetId='+encodeURIComponent(task.currentCandidateId??''),!!task.currentCandidateId);
 const result=candidate.data&&!candidate.data.stale?candidate.data.candidate.integration:undefined;
 const blocking=[...new Set(task.attention.map(blockingKey).filter((k)=>!!k))];
 return <>
  {result?<section className="drawer-section"><h3>{t('task.result')}</h3><p>{result.summary}</p><p className="muted">{t('task.untested')}</p><List items={result.untested}/></section>:null}
  {blocking.length?<section className="drawer-section"><h3>{t('task.blocking')}</h3><ul className="plain-list">{blocking.map(k=><li key={k}>{t(k)}</li>)}</ul></section>:null}
  <p className="muted">{t('task.collaborators',{count:task.collaboratorCount,limit:task.collaboratorLimit})}</p>
  {task.sourceRequestText?<section className="drawer-section"><h3>{t('task.original')}</h3><pre>{task.sourceRequestText}</pre>{task.sourceConstraints?.length?<><p className="muted">{t('task.constraints')}</p><List items={task.sourceConstraints}/></>:null}</section>:null}
  {task.recordRefs?.length?<section className="drawer-section"><h3>{t('task.records')}</h3><ul className="plain-list">{task.recordRefs.map(r=><li key={r.bindingId}><a href={r.url} target="_blank" rel="noreferrer">{t('task.record',{number:r.number})}</a></li>)}</ul></section>:null}
  <TechnicalDetails>
   <p>task {task.id} · session {task.integratorSessionId??'—'} · epoch {task.integratorEpoch} · spec v{task.specVersion} · acceptance v{task.acceptancePolicyVersion} · task v{task.version}</p>
   <h4>{t('task.scope')}</h4><List items={task.scope}/>
   <h4>{t('task.nonScope')}</h4><List items={task.nonScope}/>
   <h4>{t('task.conditions')}</h4><List items={task.acceptancePolicy.conditions}/>
   <h4>{t('task.authority')}</h4><List items={task.authorityRefs}/>
   <h4>{t('task.attention')}</h4><List items={task.attention}/>
  </TechnicalDetails>
 </>;
}
