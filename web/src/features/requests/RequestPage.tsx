import {useCollection,useResource} from '../../api/resource';
import {type Schema} from '../../api/client';
import {Feedback,List,when,memberName,type Participant} from '../shared';
import {requestStates,type WorkRequest} from './RequestPanel';
import {RequestManagement} from './RequestManagement';
export function RequestPage({id}:{id:string}){
 const record=useResource<WorkRequest>('/api/v1/queries/request?targetId='+encodeURIComponent(id));const r=record.data;
 const members=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(r?.projectId??''),!!r),sessions=useCollection<Schema['SessionView']>('/api/v1/queries/sessions?projectId='+encodeURIComponent(r?.projectId??''),!!r);
 if(!r)return <section className="surface"><p>正在讀取需求。</p><Feedback error={record.error}/></section>;
 const responsible=members.data?.find(m=>m.id===r.preparerParticipantId),session=sessions.data?.find(s=>s.id===r.preparerSessionId);
 return <section className="surface"><p className="eyebrow">需求 · {requestStates[r.state]}</p><h2>{r.text}</h2>{r.taskId?<><p>需求已整理為正式任務，後續進度由任務記錄。</p><a className="button" href={'/tasks/'+r.taskId} data-local>查看任務進度</a></>:<p>{r.state==='waiting'?'等待 agent 接手整理':r.state==='cancelled'?'已取消，原文及紀錄保留':responsible?memberName(responsible,members.data??[])+' 負責整理':'已有 agent 負責整理'}</p>}
 {!r.taskId&&r.state!=='cancelled'?<p className="muted">{session?'最近同步：'+when(session.lastSeenAt)+(session.status==='ended'?' · 工作階段已結束，等待明確續接':''):'請在原本的 agent 對話開始或繼續工作。服務不會啟動宿主。'}</p>:null}
 <List items={r.constraints}/><a href="/decisions" data-local>查看待我處理</a>
 {!['converted','cancelled'].includes(r.state)?<RequestManagement request={r} members={members.data??[]} sessions={sessions.data??[]} fresh={record.fresh&&sessions.fresh&&members.fresh} refresh={record.refresh}/>:null}
 <details><summary>原始記錄與診斷</summary><p>建立時間：{when(r.createdAt)} · 版本 {r.version}</p><pre className="mono">{JSON.stringify(r,null,2)}</pre></details><Feedback error={record.error}/>
 </section>;
}
