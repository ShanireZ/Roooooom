import {useState} from 'react';
import {useCollection} from '../../api/resource';
import {command} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {navigate} from '../../App';
import {Field,Area,Select,Feedback,lines,useAction,memberName,when,type Project,type Task,type Participant} from '../shared';
import {RequestPanel,requestStates,type WorkRequest} from '../requests/RequestPanel';
import type {Schema} from '../../api/client';
import {TaskBoard} from '../tasks/TaskBoard';
import {ThreadPanel} from '../chat/ThreadPanel';

export function ProjectsPage(){
 const projects=useCollection<Project>('/api/v1/queries/projects');
 const [selection,setSelection]=useState(''),[showForm,setShowForm]=useState(false),[showRequest,setShowRequest]=useState(false),[participant,setParticipant]=useState(''),[attention,setAttention]=useState(false),[history,setHistory]=useState(false);
 const projectId=selection||(projects.data?.length===1?projects.data[0].id:'');
 const roster=useCollection<Participant>('/api/v1/queries/participants'+(projectId?'?projectId='+encodeURIComponent(projectId):''));
 const names=Object.fromEntries(roster.data?.map(p=>[p.id,memberName(p,roster.data??[])])??[]);
 const requests=useCollection<WorkRequest>('/api/v1/queries/requests'+(projectId?'?projectId='+encodeURIComponent(projectId):''));
 const sessions=useCollection<Schema['SessionView']>('/api/v1/queries/sessions'+(projectId?'?projectId='+encodeURIComponent(projectId):''));
 const path='/api/v1/queries/tasks'+(projectId?'?projectId='+encodeURIComponent(projectId):'');
 const tasks=useCollection<Task>(path);
 const events=useWorkspaceEvents();
 const selected=projects.data?.find(x=>x.id===projectId);
 const visible=tasks.data?.filter(t=>(history||t.phase!=='cancelled')&&(!participant||t.participantIds?.includes(participant))&&(!attention||t.attention.length>0))??[];
 const participantIds=[...new Set(tasks.data?.flatMap(t=>t.participantIds??[])??[])];
 return <>
  <div className="toolbar"><div className="filters">
   <Select label="專案篩選" value={projectId} onChange={e=>{setSelection(e.target.value);setParticipant('')}}><option value="">全部專案</option>{projects.data?.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</Select>
   <Select label="參與 agent" value={participant} onChange={e=>setParticipant(e.target.value)}><option value="">全部 agent</option>{participantIds.map(id=><option key={id} value={id}>{names[id]??id} · {id.slice(0,8)}</option>)}</Select>
   <label className="choice"><input type="checkbox" checked={attention} onChange={e=>setAttention(e.target.checked)}/><span>只看需關注</span></label>
   <label className="choice"><input type="checkbox" checked={history} onChange={e=>setHistory(e.target.checked)}/><span>顯示取消與歷史</span></label>
  </div><div className="actions"><a href="/setup/new" data-local>接入專案</a><button disabled={!events.connected||!projects.fresh||!projects.data?.length} onClick={()=>setShowRequest(x=>!x)} aria-expanded={showRequest}>新增需求</button></div></div>
  <Feedback error={projects.error??tasks.error}/>
  {showForm?<TaskForm projects={projects.data??[]} projectId={projectId||projects.data?.[0]?.id||''} onCancel={()=>setShowForm(false)}/>:null}
  {showRequest?<RequestPanel projects={projects.data??[]} projectId={projectId||projects.data?.[0]?.id||''} onCancel={()=>setShowRequest(false)} onDone={()=>{setShowRequest(false);requests.refresh()}}/>:null}
  <section className="project-summary" data-testid="project-summary" aria-label="專案進度">{projects.data?.filter(p=>!projectId||p.id===projectId).map(project=>{
   const group=tasks.data?.filter(t=>t.projectId===project.id)??[];
   return <article key={project.id}><h2>{project.name}</h2><p><strong>{group.filter(t=>t.phase==='done').length}</strong> / {group.length} 個任務已完成</p><span className="muted">{group.filter(t=>t.attention.length).length} 個需關注 · {group.filter(t=>['active','integrating','review'].includes(t.phase)).length} 個推進中</span></article>;
  })}</section>
  {!projects.data?.length?<section className="surface empty"><h2>目前尚無專案</h2><p>接入第一個專案，接著把說明交給原本的 agent。</p><a className="button" href="/setup/new" data-local>接入專案</a></section>:<TaskBoard tasks={visible} names={names}/>}
  {projects.data?.length?<><section className="surface"><h2>需求</h2>{requests.data?.filter(r=>history||!['converted','cancelled'].includes(r.state)).map(r=><article key={r.id} className="request-row"><a href={r.taskId?'/tasks/'+r.taskId:'/requests/'+r.id} data-local><h3>{r.text}</h3></a><p className="muted">{requestStates[r.state]} · {r.preparerParticipantId?names[r.preparerParticipantId]??'整理 agent':'等待 agent 接手整理'}</p></article>)}{!requests.data?.some(r=>history||!['converted','cancelled'].includes(r.state))?<p className="muted">沒有待整理需求。用一句話留下想做的事。</p>:null}<details><summary>已備妥完整規格？</summary><button className="secondary" disabled={!events.connected} onClick={()=>setShowForm(x=>!x)}>建立任務</button></details></section>
  <details className="surface" data-testid="project-members"><summary>成員與最近同步</summary>{roster.data?.map(p=><p key={p.id}>{memberName(p,roster.data??[])} · agent{sessions.data?.filter(s=>s.participantId===p.id).map(s=><span key={s.id}> · 最近同步 {when(s.lastSeenAt)}{s.status==='ended'?' · 已結束':''}</span>)}</p>)}{!roster.data?.length?<p className="muted">尚無 agent，請把接入說明交給原本的對話。</p>:null}{selected?<a href={'/setup/'+selected.id} data-local>複製接入說明</a>:null}</details></>:null}
  {selected?<ThreadPanel projectId={selected.id}/>:null}
 </>;
}
function TaskForm({projects,projectId,onCancel}:{projects:Project[];projectId:string;onCancel:()=>void}){
 const [project,setProject]=useState(projectId),[mode,setMode]=useState('integrator');
 const participants=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(project),!!project);
 const action=useAction(),events=useWorkspaceEvents();
 return <section id="new-task" className="surface"><h2>登記新的協作任務</h2><p className="muted">N 是額外協作者上限。任務登記後由 agent 主動接手。</p>
  <form action="/api/v1/commands/task.create" method="post" onSubmit={e=>{
   e.preventDefault();const form=new FormData(e.currentTarget);
   void action.run(async()=>{
    const payload={projectId:project,goal:String(form.get('goal')),scope:lines(String(form.get('scope'))),nonScope:lines(String(form.get('nonScope'))),authorityRefs:lines(String(form.get('authorityRefs'))),risk:String(form.get('risk')),collaboratorLimit:Number(form.get('limit')),designatedIntegratorId:String(form.get('integrator')??'')||undefined,acceptancePolicy:{mode,acceptorId:mode==='designated-agent'?String(form.get('acceptor')):undefined,conditions:lines(String(form.get('conditions')))}};
    const response=await command<Task>('task.create',payload);navigate('/tasks/'+response.result.id);
   });
  }}>
   <fieldset disabled={!events.connected||action.busy}><legend className="sr-only">任務目標與協作約定</legend>
    <Select label="所屬專案" name="project" value={project} onChange={e=>setProject(e.target.value)}>{projects.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</Select>
    <Field label="任務目標" name="goal" required maxLength={8192}/>
    <Area label="範圍（每行一項）" name="scope" required/>
    <Area label="不包含（每行一項）" name="nonScope"/>
    <Area label="權威引用（每行一項）" name="authorityRefs" required spellCheck={false}/>
    <div className="form-row"><Field label="額外協作者上限 N" name="limit" type="number" min={0} max={64} defaultValue={0} required/>
     <Select label="風險" name="risk" defaultValue="normal"><option value="normal">一般</option><option value="low">低</option><option value="high">高：開工前需我確認</option></Select></div>
    <Select label="指定整合 agent（選填）" name="integrator" defaultValue=""><option value="">首個承接整合責任者</option>{participants.data?.map(p=><option key={p.id} value={p.id}>{memberName(p,participants.data??[])}</option>)}</Select>
    <Select label="驗收方式" name="mode" value={mode} onChange={e=>setMode(e.target.value)}><option value="integrator">整合 agent 驗收</option><option value="human">由我人工驗收</option><option value="designated-agent">指定 agent 驗收</option></Select>
    {mode==='designated-agent'?<Select label="指定驗收 agent" name="acceptor" required><option value="">請選擇</option>{participants.data?.map(p=><option key={p.id} value={p.id}>{memberName(p,participants.data??[])}</option>)}</Select>:null}
    <Area label="驗收條件（每行一項）" name="conditions" required/>
    <div className="actions"><button type="submit">{action.busy?'登記中…':'登記任務'}</button><button type="button" className="secondary" onClick={onCancel}>取消</button></div>
   </fieldset><Feedback error={action.error}/>
  </form>
 </section>;
}
