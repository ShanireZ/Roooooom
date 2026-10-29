import {useResource,useCollection} from '../../api/resource';
import {AssignmentPanel} from './AssignmentPanel';
import {ThreadPanel} from '../chat/ThreadPanel';
import {DeliveryPanel} from '../deliveries/DeliveryPanel';
import {Feedback,List,phases,memberName,type Task,type Project,type Participant} from '../shared';
import {TaskManagement} from './TaskManagement';
import type {Schema} from '../../api/client';
export function TaskPage({id}:{id:string}){
 const task=useResource<Task&{sourceRequestText?:string;sourceConstraints?:string[]}>('/api/v1/queries/task?targetId='+encodeURIComponent(id));
 const projects=useCollection<Project>('/api/v1/queries/projects');
 const roster=useCollection<Participant>('/api/v1/queries/participants');
 const result=useResource<Schema['CandidateStatusView']>('/api/v1/queries/candidate?targetId='+encodeURIComponent(task.data?.currentCandidateId??''),!!task.data?.currentCandidateId);
 if(!task.data)return <section className="surface"><p>{task.error?'無法讀取任務：'+task.error:'正在讀取任務。'}</p></section>;
 const t=task.data,project=projects.data?.find(p=>p.id===t.projectId),integrator=roster.data?.find(p=>p.id===t.integratorId);
 return <>
  <section className="surface task-overview"><div className="section-heading"><h2>{t.goal}</h2><span className="phase-label" data-testid="task-phase">{phases[t.phase]??t.phase}</span></div>
   <div className="task-meta"><span>協作者 {t.collaboratorCount} / {t.collaboratorLimit}</span><span>{t.phase==='done'?'已完成':t.canStart?'可按依賴推進':'需先處理阻塞或責任'}</span></div>
   <p>整合者：{integrator?memberName(integrator,roster.data??[]):'等待整合 agent 接手'}</p><p className="muted">{t.attention.includes('integrator-session-ended')||t.attention.includes('integrator-presence-stale')?'等待 agent 回來同步。請在原宿主繼續對話。':t.acceptancePolicy.mode==='human'?'結果準備好後由你驗收。':'整合完成後由指定責任者驗收。'}</p>
   {result.data&&!result.data.stale?<div className="inset"><h3>目前成果</h3><p>{result.data.candidate.integration.summary}</p><p className="muted">未驗證範圍</p><List items={result.data.candidate.integration.untested}/></div>:null}
   {t.sourceRequestText?<details><summary>你的原始需求與要求</summary><pre>{t.sourceRequestText}</pre><List items={t.sourceConstraints}/></details>:null}
   <details><summary>完整規格與診斷</summary><p className="mono">task {t.id} · session {t.integratorSessionId??'尚未接手'} · epoch {t.integratorEpoch} · spec {t.specVersion} · acceptance {t.acceptancePolicyVersion}</p>
   <div className="three-columns"><div><h3>範圍</h3><List items={t.scope}/></div><div><h3>不包含</h3><List items={t.nonScope}/></div><div><h3>驗收條件</h3><List items={t.acceptancePolicy.conditions}/></div></div>
   <details><summary>權威引用及需關注項</summary><List items={t.authorityRefs}/><List items={t.attention}/></details>
   </details>
   <div className="actions"><a href="/decisions" data-local>前往正式拍板／驗收</a></div>
   {t.recordRefs?.length?<ul className="plain-list">{t.recordRefs.map(r=><li key={r.bindingId}><a href={r.url} target="_blank" rel="noreferrer">GitHub 紀錄 #{r.number}</a> · 建立時任務 v{r.taskVersion}</li>)}</ul>:null}
   <TaskManagement key={t.id} task={t} fresh={task.fresh} refresh={task.refresh}/>
  </section>
  <Feedback error={task.error}/>
  {project?<AssignmentPanel task={t} project={project} fresh={task.fresh}/>:null}
  <ThreadPanel projectId={t.projectId} task={t}/>
  <details className="surface"><summary>交付、測試與歷史證據</summary><DeliveryPanel task={t}/></details>
 </>;
}
