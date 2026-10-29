import {useId,useRef,useState} from 'react';
import {useCollection,useResource} from '../../api/resource';
import {command,type Schema} from '../../api/client';
import {Area,Field,Select,Toggle,Feedback,List,lines,useAction,versions,when,memberName,type Task,type Participant} from '../shared';
import type {WorkRequest} from '../requests/RequestPanel';
type Question=Schema['DecisionRequestView'];

export function DecisionPage(){
 const [notice,setNotice]=useState(''),[historyOpen,setHistoryOpen]=useState(false);
 const saved=(message:string)=>{setNotice(message);setHistoryOpen(true)};
 const requests=useCollection<Question>('/api/v1/queries/decisions');
 const tasks=useCollection<Task>('/api/v1/queries/tasks');
 const pending=requests.data?.filter(r=>r.state==='pending')??[],history=requests.data?.filter(r=>r.state!=='pending')??[];
 return <>
  <p className="page-description">在這裡送出你的正式回覆及驗收。聊天中的轉述會保留其原作者身分。</p>
  <Feedback error={requests.error??tasks.error} notice={notice}/>
  <section aria-label="待我決策"><h2>待拍板 · {pending.length}</h2>{pending.map(r=><DecisionCard key={r.id} request={r} onSaved={saved}/>)}{!pending.length?<p className="muted">目前沒有待決問題。</p>:null}</section>
  <section aria-label="人工驗收"><h2>人工驗收</h2>{tasks.data?.filter(t=>t.acceptancePolicy.mode==='human'&&t.currentCandidateId&&!['done','cancelled'].includes(t.phase)).map(task=><HumanAcceptance key={task.id} task={task} fresh={tasks.fresh}/>)}</section>
  <section className="surface"><h2>需要你處理的阻塞</h2>{tasks.data?.filter(t=>!['done','cancelled'].includes(t.phase)&&t.attention.some(a=>a==='integrator-revoked'||a==='integrator-session-ended'||a.startsWith('pending-reconciliation:'))).map(t=><p key={t.id}><a href={'/tasks/'+t.id} data-local>{t.goal}</a> · 需要明確續接、交接或核對離線紀錄</p>)}<p className="muted">一般待接手的工作留在專案看板。</p></section>
  <details className="surface"><summary>調整驗收規則</summary><p className="muted">你指定的規則只能由你更改；改版會讓舊候選失效。</p>{tasks.data?.map(task=><details key={task.id} className="evidence"><summary>{task.goal}</summary><PolicyEditor key={task.id+'|'+task.version} task={task} fresh={tasks.fresh}/></details>)}</details>
  <details className="surface" open={historyOpen} onToggle={e=>setHistoryOpen(e.currentTarget.open)}><summary>歷史決策 · {history.length}</summary>{history.map(r=><DecisionCard key={r.id} request={r} onSaved={saved}/>)}</details>
 </>;
}
function DecisionCard({request,onSaved}:{request:Question;onSaved:(message:string)=>void}){
 const detail=useResource<Omit<Schema['DecisionHistoryView'],'request'>&{request:Question}>('/api/v1/queries/decision?targetId='+encodeURIComponent(request.id));
 const r=detail.data?.request??request;
 const [edit,setEdit]=useState(false);
 const intake=useResource<WorkRequest>('/api/v1/queries/request?targetId='+encodeURIComponent(r.requestId??''),!!r.requestId);
 const taskId=r.taskId||intake.data?.taskId;
 const task=useResource<Task>('/api/v1/queries/task?targetId='+encodeURIComponent(taskId??''),!!taskId);
 return <article className="surface decision-card"><div className="section-heading"><h3>{r.question}</h3><span className="muted">問題 v{r.version} · {r.state}</span></div>
  <p>{r.reason}</p><p><strong>建議：</strong>{r.options.find(o=>o.id===r.recommendation)?.label} · {r.recommendationReason}</p><p>影響：{r.impact}</p>
  {r.externalProposal?<div className="notice"><strong>待確認的外部動作：{r.externalProposal.action}</strong><span>目標：{r.externalProposal.target}</span><span>範圍：{r.externalProposal.scope}</span><span>最多 {r.externalProposal.maxUses} 次 · 到期 {when(r.externalProposal.expiresAt)}</span><span>這是 agent 準備的內容，尚未授權。</span></div>:null}
  <div className="three-columns"><div><h3>依據</h3><List items={r.factRefs}/></div><div><h3>阻塞</h3><List items={[...r.blockedActions,...r.blockedAssignmentIds]}/></div><div><h3>可繼續</h3><List items={r.canContinue}/></div></div>
  <p className="muted">未回覆時：{r.noDecisionBehavior}{r.deadline?' · 截止 '+when(r.deadline):''}</p><a href={taskId?'/tasks/'+taskId:'/requests/'+r.requestId} data-local>{taskId?'查看任務':'查看原始需求'}</a>
  {r.followUpRequired&&!r.followUpComplete?<p className="notice">正式決策已保存，仍等待權威文件更新的證據。</p>:null}
  {r.state==='pending'||edit?<DecisionForm key={r.id+'|'+r.version} request={r} task={task.data} fresh={detail.fresh&&(taskId?task.fresh:intake.fresh)} done={message=>{setEdit(false);detail.refresh();onSaved(message)}}/>:r.state==='resolved'?<button className="secondary" onClick={()=>setEdit(true)}>修訂正式決策</button>:null}
  {detail.data?.history.map(decision=><div className="formal-decision" data-testid="formal-decision" key={decision.id}><strong>human 正式決策 · v{decision.version}</strong><pre>{decision.answer}</pre><p className="muted">{decision.outcome} · {when(decision.receivedAt)} · {decision.id}</p></div>)}
  <Feedback error={detail.error}/>
 </article>;
}
function DecisionForm({request:r,task,fresh,done}:{request:Question;task?:Task;fresh:boolean;done:(message:string)=>void}){
 const action=useAction(),group=useId();
 const [answer,setAnswer]=useState(''),[option,setOption]=useState<string|undefined>(undefined),[amend,setAmend]=useState(false),[authorize,setAuthorize]=useState(false),[withdrawing,setWithdrawing]=useState(false);
 const draft=r.specProposal??task;
 return <form className="inset" action={'/api/v1/commands/'+(r.state==='pending'?'decision.resolve':'decision.supersede')} method="post" onSubmit={e=>{
  e.preventDefault();const form=new FormData(e.currentTarget),submitter=(e.nativeEvent as SubmitEvent).submitter as HTMLButtonElement|null;
  void action.run(async()=>{
   const withdraw=submitter?.value==='withdraw';
   const visible=r.options.find(o=>o.id===option)?.label;
   const formalAnswer=visible?(visible+(answer.trim()?'\n補充：'+answer.trim():'')):answer.trim();
   const payload:Record<string,unknown>=withdraw?{reason:answer}:{answer:formalAnswer,optionId:option||undefined,supersedesDecisionId:r.state==='resolved'?r.currentDecisionId:undefined,authorityFollowUpRequired:form.get('followup')==='on',outcome:option?undefined:String(form.get('outcome'))};
   if(amend&&!withdraw)payload.specAmendment={goal:String(form.get('amendGoal')),scope:lines(String(form.get('amendScope'))),nonScope:lines(String(form.get('amendNonScope'))),authorityRefs:lines(String(form.get('amendAuthority'))),risk:String(form.get('amendRisk'))};
   if(authorize&&!withdraw)payload.externalAuthorization={action:String(form.get('externalAction')),target:String(form.get('externalTarget')),scope:String(form.get('externalScope')),originalText:String(form.get('externalText')),expiresAt:new Date(String(form.get('expiresAt'))).toISOString(),maxUses:Number(form.get('maxUses'))};
   await command(withdraw?'decision.withdraw':r.state==='pending'?'decision.resolve':'decision.supersede',payload,r.id,{expectedVersion:r.version});
   done(withdraw?'待決事項已撤回，理由已記錄。':'正式決策已保存，可在歷史決策查看目前版本。');
  });
 }}><fieldset disabled={!fresh||action.busy}><legend>{r.state==='pending'?'你的正式回覆':'修訂目前正式決策'}</legend>
  {r.options.map(o=>{const id=group+'-'+o.id;return <label className="choice option" key={o.id} htmlFor={id}><input id={id} name={group} type="radio" value={o.id} checked={option===o.id} onChange={()=>setOption(o.id)}/><span><strong>{o.label}</strong><small>{o.impact}</small></span></label>})}
  <label className="choice" htmlFor={group+'-custom'}><input id={group+'-custom'} name={group} type="radio" checked={option===''} onChange={()=>setOption('')}/><span>輸入不同的決定</span></label>
  {option===''?<Select label="決定結果" name="outcome" defaultValue="answered"><option value="answered">已回覆</option><option value="approved">批准</option><option value="rejected">拒絕</option><option value="revise">要求修訂</option></Select>:null}
  <Area label={withdrawing?'撤回理由':option===''?'正式回覆':'補充說明（選填）'} name="answer" value={answer} onChange={e=>setAnswer(e.target.value)} required={withdrawing||option===''} maxLength={8192}/>
  <details><summary>權威文件與修訂要求</summary><Toggle label="需要更新專案權威" name="followup"/></details>
  {task&&r.blockedActions.includes('spec-change')?<><Toggle label="同時修訂任務規格" checked={amend} onChange={e=>setAmend(e.target.checked)}/>{amend?<div className="inset"><Field label="修訂後目標" name="amendGoal" defaultValue={draft?.goal} required/><Area label="修訂後範圍" name="amendScope" defaultValue={draft?.scope.join('\n')} required/><Area label="修訂後不包含" name="amendNonScope" defaultValue={draft?.nonScope.join('\n')}/><Area label="修訂後權威引用" name="amendAuthority" defaultValue={draft?.authorityRefs.join('\n')} required/><Select label="修訂後風險" name="amendRisk" defaultValue={draft?.risk}><option value="low">低</option><option value="normal">一般</option><option value="high">高</option></Select></div>:null}</>:null}
  {r.blockedActions.includes('external-action')?<><Toggle label="核發本次對外動作授權" checked={authorize} onChange={e=>setAuthorize(e.target.checked)}/>{authorize?<div className="inset"><p>此處保存授權範圍；服務不執行 Git、PR 或部署。</p><Select label="對外動作" name="externalAction" defaultValue={r.externalProposal?.action} required>{['push','pr','merge','release','deploy'].map(x=><option key={x}>{x}</option>)}</Select><Field label="授權目標" name="externalTarget" defaultValue={r.externalProposal?.target} identifier required/><Area label="授權範圍" name="externalScope" defaultValue={r.externalProposal?.scope} required/><Area label="你確認的授權文字" name="externalText" defaultValue={r.externalProposal?.originalText} required/><Field label="授權到期時間" name="expiresAt" type="datetime-local" defaultValue={r.externalProposal?new Date(new Date(r.externalProposal.expiresAt).getTime()-new Date().getTimezoneOffset()*60000).toISOString().slice(0,16):undefined} required/><Field label="允許使用次數" name="maxUses" type="number" min={1} max={100} defaultValue={r.externalProposal?.maxUses??1} required/></div>:null}</>:null}
  <div className="actions">{withdrawing?<><button className="danger" type="submit" name="action" value="withdraw">確認撤回待決事項</button><button type="button" className="secondary" onClick={()=>setWithdrawing(false)}>返回選擇</button></>:<><button type="submit" disabled={option===undefined}>送出正式決策</button>{r.state==='pending'?<button className="secondary" type="button" onClick={()=>{setWithdrawing(true);setAnswer('')}}>撤回待決事項</button>:null}</>}</div>
 </fieldset><Feedback error={action.error}/></form>;
}
function HumanAcceptance({task,fresh}:{task:Task;fresh:boolean}){
 const candidate=useResource<Schema['CandidateStatusView']>('/api/v1/queries/candidate?targetId='+encodeURIComponent(task.currentCandidateId??''),!!task.currentCandidateId);
 const accepted=useCollection<Schema['AcceptanceView']>('/api/v1/queries/acceptances?targetId='+encodeURIComponent(task.id));
 const action=useAction(),c=candidate.data?.candidate;
 const already=accepted.data?.some(a=>a.candidateId===task.currentCandidateId&&a.actorKind==='human');
 return <article className="surface"><h3>{task.goal}</h3>{!c?<p className="muted">尚無有效驗收候選</p>:<>
  <p className="muted">候選 <span className="mono">{c.id}</span> · 規格 v{c.integration.specVersion} · 驗收 v{c.integration.acceptancePolicyVersion}</p>
  <p>{c.integration.summary}</p><ul className="plain-list">{c.integration.heads.map(h=><li className="mono" key={h.repoId}>{h.repoId} @ {h.headSha}</li>)}</ul>
  {c.integration.checks.map(check=><div key={check.condition}><strong>{check.condition} · {check.outcome}</strong><List items={check.evidenceRefs}/></div>)}
  <p className="muted">上述證據由 agent 回報，接受會綁定這份精確整合版本。</p>
  <h4>未驗證範圍</h4><List items={c.integration.untested}/>
  {candidate.data?.stale?<p className="notice">此候選已失效，請等待重新整合。</p>:already?<p className="success">你已接受此版本，等待整合者結案。</p>:<button disabled={!fresh||!candidate.fresh||!accepted.fresh||action.busy} onClick={()=>void action.run(async()=>{await command('acceptance.accept',{candidateId:c.id},task.id,versions(task));accepted.refresh()})}>接受此驗收版本</button>}
 </>}<Feedback error={action.error??candidate.error??accepted.error}/><a className="acceptance-link" href={'/tasks/'+task.id} data-local>查看完整任務及歷史證據</a></article>;
}
function PolicyEditor({task,fresh}:{task:Task;fresh:boolean}){
 const base=useRef(versions(task)).current,action=useAction(),[mode,setMode]=useState<string>(task.acceptancePolicy.mode);
 const members=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(task.projectId));
 return <form action="/api/v1/commands/acceptance.policy" method="post" onSubmit={e=>{
  e.preventDefault();const form=new FormData(e.currentTarget);
  void action.run(async()=>{await command('acceptance.policy',{policy:{mode,acceptorId:mode==='designated-agent'?String(form.get('acceptor')):undefined,conditions:lines(String(form.get('conditions')))},reason:String(form.get('reason'))},task.id,base);action.setNotice('驗收規則已更新，舊候選失效。')});
 }}><fieldset disabled={!fresh||action.busy}><legend>由你調整驗收規則</legend><Select label="新的驗收方式" value={mode} onChange={e=>setMode(e.target.value)}><option value="integrator">整合 agent</option><option value="human">由我驗收</option><option value="designated-agent">指定 agent</option></Select>{mode==='designated-agent'?<Select label="新的驗收 agent" name="acceptor" defaultValue={task.acceptancePolicy.acceptorId} required><option value="">請選擇</option>{members.data?.map(p=><option key={p.id} value={p.id}>{memberName(p,members.data??[])}</option>)}</Select>:null}<Area label="新的驗收條件" name="conditions" defaultValue={task.acceptancePolicy.conditions.join('\n')} required/><Field label="修改驗收規則的理由" name="reason" required/><button type="submit">更新驗收規則</button></fieldset><Feedback error={action.error} notice={action.notice}/></form>;
}
