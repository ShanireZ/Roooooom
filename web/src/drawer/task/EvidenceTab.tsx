import type {Schema} from '../../api/client';
import {useCollection,useResource} from '../../api/resource';
import {useI18n} from '../../i18n/I18nProvider';
import {TechnicalDetails} from '../../ui/Fold';
import {RelativeTime} from '../../ui/RelativeTime';
import {List} from './OverviewTab';
import type {TaskView} from './TaskDrawer';

// Agent-reported evidence as plain summaries; exact heads and checks in folds.
export function EvidenceTab({task,names}:{task:TaskView;names:Record<string,string>}){
 const {t,tn}=useI18n(),suffix='?targetId='+encodeURIComponent(task.id);
 const deliveries=useCollection<Schema['DeliveryView']>('/api/v1/queries/deliveries'+suffix);
 const integrations=useCollection<Schema['IntegrationView']>('/api/v1/queries/integrations'+suffix);
 const candidates=useCollection<Schema['CandidateView']>('/api/v1/queries/candidates'+suffix);
 const completions=useCollection<Schema['CompletionView']>('/api/v1/queries/completions'+suffix);
 const current=useResource<Schema['CandidateStatusView']>('/api/v1/queries/candidate?targetId='+encodeURIComponent(task.currentCandidateId??''),!!task.currentCandidateId);
 const who=(id:string,kind:string)=>kind==='human'?t('activity.you'):names[id]??t('seat.revoked');
 if(!deliveries.data?.length&&!integrations.data?.length)return <p className="muted">{t('evidence.empty')}</p>;
 return <>
  <p className="muted">{t('evidence.reported')}</p>
  <ul className="record-list" aria-label={t('task.tab.evidence')}>
   {deliveries.data?.map(d=><li key={d.id}>
    <p>{t('evidence.delivery',{actor:who(d.actorId,d.actorKind),revision:d.revision,summary:d.summary})} · {tn('evidence.checks',d.evidenceRefs.length)} · <RelativeTime iso={d.receivedAt}/></p>
    <TechnicalDetails><p>{d.repoId} · {d.baseSha} → {d.headSha} · {d.branchRef}</p><p>{d.id} · assignment {d.assignmentId} · spec v{d.specVersion} · contract v{d.assignmentContractVersion} · claim epoch {d.claimEpoch}</p><List items={d.evidenceRefs}/><h4>{t('evidence.unknowns')}</h4><List items={d.unknowns}/><List items={d.artifactRefs.map(a=>a.reference+' · '+a.digest)}/></TechnicalDetails>
   </li>)}
   {integrations.data?.map(i=><li key={i.id}>
    <p>{t('evidence.integration',{actor:who(i.actorId,i.actorKind),summary:i.summary})} · {i.result}</p>
    <TechnicalDetails><p>{i.id} · spec v{i.specVersion} · acceptance v{i.acceptancePolicyVersion} · integrator epoch {i.integratorEpoch}</p><List items={i.heads.map(h=>h.repoId+' @ '+h.headSha)}/>{i.checks.map(c=><div key={c.condition}><strong>{c.condition} · {c.outcome}</strong><List items={c.evidenceRefs}/></div>)}<h4>{t('evidence.unknowns')}</h4><List items={i.untested}/></TechnicalDetails>
   </li>)}
   {candidates.data?.map(c=><li key={c.id} className="muted">{t('evidence.candidate')} · {c.id===task.currentCandidateId&&!current.data?.stale?t('evidence.candidateCurrent'):t('evidence.candidateStale')}<TechnicalDetails><p>{c.id}</p></TechnicalDetails></li>)}
   {completions.data?.map(c=><li key={c.id}>{t('evidence.completion')} · <RelativeTime iso={c.receivedAt}/><TechnicalDetails><p>{c.id} · {c.candidate.id} · {c.actorKind} {c.actorId}</p></TechnicalDetails></li>)}
  </ul>
 </>;
}
