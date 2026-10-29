import {useCollection,useResource} from '../../api/resource';
import type {Schema} from '../../api/client';
import {List,when,type Task} from '../shared';
export function DeliveryPanel({task}:{task:Task}){
 const suffix='?targetId='+encodeURIComponent(task.id);
 const deliveries=useCollection<Schema['DeliveryView']>('/api/v1/queries/deliveries'+suffix);
 const integrations=useCollection<Schema['IntegrationView']>('/api/v1/queries/integrations'+suffix);
 const candidates=useCollection<Schema['CandidateView']>('/api/v1/queries/candidates'+suffix);
 const completions=useCollection<Schema['CompletionView']>('/api/v1/queries/completions'+suffix);
 const current=useResource<Schema['CandidateStatusView']>('/api/v1/queries/candidate?targetId='+encodeURIComponent(task.currentCandidateId??''),!!task.currentCandidateId);
 return <section className="surface"><h2>交付與整合證據</h2><p className="muted">以下 Git、檢查與測試資料由 agent 回報。服務核對引用及版本。</p>
  {deliveries.data?.map(d=><details className="evidence" key={d.id}><summary>交付 r{d.revision} · {d.summary}</summary><p className="mono">{d.id} · assignment {d.assignmentId}</p><p>作者 {d.actorKind} · {d.actorId} · {when(d.receivedAt)}</p><p className="mono">{d.repoId} · {d.baseSha} → {d.headSha}</p><p className="muted">規格 v{d.specVersion} · contract v{d.assignmentContractVersion} · claim epoch {d.claimEpoch}</p><List items={d.evidenceRefs}/><h3>未核驗</h3><List items={d.unknowns}/><List items={d.artifactRefs.map(a=>a.reference+' · '+a.digest)}/></details>)}
  {integrations.data?.map(i=><details className="evidence" key={i.id}><summary>整合 attempt · {i.result} · {i.summary}</summary><p className="mono">{i.id}</p><p className="muted">規格 v{i.specVersion} · 驗收 v{i.acceptancePolicyVersion} · integrator epoch {i.integratorEpoch}</p><ul className="plain-list">{i.heads.map(h=><li key={h.repoId} className="mono">{h.repoId} @ {h.headSha}</li>)}</ul>{i.checks.map(c=><div key={c.condition}><strong>{c.condition} · {c.outcome}</strong><List items={c.evidenceRefs}/></div>)}<h3>未測範圍</h3><List items={i.untested}/></details>)}
  {candidates.data?.map(c=><p key={c.id} className="muted">驗收候選 <span className="mono">{c.id}</span> · {c.id===task.currentCandidateId&&!current.data?.stale?'目前候選':'歷史版本／已失效'}</p>)}
  {completions.data?.map(c=><details className="evidence" key={c.id}><summary>完成快照 · {when(c.receivedAt)}</summary><p className="mono">{c.id} · {c.candidate.id}</p><p>結案者：{c.actorKind} · {c.actorId}</p><p className="muted">快照保留，後續改版可能讓任務重新進入審閱。</p></details>)}
  {!deliveries.data?.length&&!integrations.data?.length?<p className="muted">尚無交付。worker 提交後仍由整合者接收並驗證。</p>:null}
  {[deliveries.error,integrations.error,candidates.error,completions.error].filter(Boolean).map((e,i)=><p key={i} className="error" role="alert">{e}</p>)}
 </section>;
}
