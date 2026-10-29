import {command,type Schema} from '../../api/client';
import {useCollection,useResource} from '../../api/resource';
import {navigate} from '../../App';
import {useI18n} from '../../i18n/I18nProvider';
import {routePath} from '../../shell/routes';
import {Feedback} from '../../ui/Feedback';
import {Fold,TechnicalDetails} from '../../ui/Fold';
import {Stamp} from '../../ui/Stamp';
import {useAction} from '../../ui/useAction';
import {Drawer} from '../Drawer';
import {List} from '../task/OverviewTab';
import {useNames} from '../task/TaskDrawer';
import {versions} from '../task/versions';

const mark=(outcome?:string)=>outcome==='passed'?'✓':outcome==='failed'?'✗':'–';
// Plain summary and the owner's own conditions first; exact heads and candidate one layer down.
export function AcceptanceDrawer({projectId,taskId}:{projectId:string;taskId:string}){
 const {t}=useI18n(),action=useAction();
 const task=useResource<Schema['Task']>('/api/v1/queries/task?targetId='+encodeURIComponent(taskId));
 const candidate=useResource<Schema['CandidateStatusView']>('/api/v1/queries/candidate?targetId='+encodeURIComponent(task.data?.currentCandidateId??''),!!task.data?.currentCandidateId);
 const accepted=useCollection<Schema['AcceptanceView']>('/api/v1/queries/acceptances?targetId='+encodeURIComponent(taskId));
 const {names}=useNames(projectId);
 const close=()=>navigate(routePath({name:'room',projectId}));
 if(!task.data)return <Drawer title={task.error?t('task.notFound'):t('task.loading')} onClose={close}><Feedback error={task.error}/></Drawer>;
 const tk=task.data,c=candidate.data?.candidate,integration=c?.integration;
 const already=accepted.data?.some(a=>a.candidateId===tk.currentCandidateId&&a.actorKind==='human');
 return <Drawer title={tk.goal} onClose={close}>
  <p><Stamp kind="acceptance"/></p>
  {!c||!integration?<p className="muted">{t('acceptance.none')}</p>:<>
   <p>{integration.summary}</p>
   <h3>{t('acceptance.conditions')}</h3>
   <ul className="plain-list checklist" aria-label={t('acceptance.conditions')}>{tk.acceptancePolicy.conditions.map(condition=>{
    const check=integration.checks.find(x=>x.condition===condition);
    return <li key={condition}><span aria-hidden="true">{mark(check?.outcome)}</span> {condition} <span className="muted">· {check?check.outcome:t('acceptance.notReported')}</span></li>;
   })}</ul>
   <h3>{t('task.untested')}</h3><List items={integration.untested}/>
   <p className="muted">{t('acceptance.reportedBy',{name:names[integration.actorId]??t('seat.revoked')})} {t('acceptance.binding')}</p>
   <Fold summary={t('acceptance.evidence')}>{integration.checks.map(x=><div key={x.condition}><strong>{x.condition} · {x.outcome}</strong><List items={x.evidenceRefs}/></div>)}</Fold>
   <TechnicalDetails><p>{c.id} · spec v{integration.specVersion} · acceptance v{integration.acceptancePolicyVersion}</p><List items={integration.heads.map(h=>h.repoId+' @ '+h.headSha)}/></TechnicalDetails>
   <div className="actions">
    {candidate.data?.stale?<p className="notice">{t('acceptance.stale')}</p>
     :already?<p className="success">{t('acceptance.accepted')}</p>
     :<button type="button" disabled={!task.fresh||!candidate.fresh||!accepted.fresh||action.busy} onClick={()=>void action.run(async()=>{await command('acceptance.accept',{candidateId:c.id},tk.id,versions(tk));accepted.refresh()})}>{t('acceptance.accept')}</button>}
    <button type="button" className="btn-ghost" onClick={()=>navigate(routePath({name:'room',projectId,drawer:{kind:'task',id:tk.id,tab:'discussion'}}))}>{t('acceptance.comment')}</button>
   </div>
  </>}
  <Feedback error={action.error??candidate.error??accepted.error}/>
 </Drawer>;
}
