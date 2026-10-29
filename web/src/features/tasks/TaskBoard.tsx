import {phases,type Task} from '../shared';
export function TaskBoard({tasks,names}:{tasks:Task[];names:Record<string,string>}){
 return <div className="board-scroll"><div className="board" aria-label="任務看板">{[['ready','待接手'],['active','進行中'],['done','已完成'],...(tasks.some(t=>t.phase==='cancelled')?[['cancelled','已取消']]:[])].map(([phase,label])=>{
  const items=tasks.filter(t=>phase==='active'?['active','integrating','review'].includes(t.phase):t.phase===phase);
  return <section key={phase} className={'phase phase-'+phase} data-testid={'phase-'+phase}><h2>{label}<span>{items.length}</span></h2>{items.map(t=><a className="task-card" key={t.id} href={'/tasks/'+t.id} data-local>
   <h3>{t.goal}</h3><p className="muted">{phases[t.phase]} · 協作者 {t.collaboratorCount} / {t.collaboratorLimit}</p>
   <p className="muted">{t.integratorId?'整合者 '+(names[t.integratorId]??t.integratorId.slice(0,9)+'…'):'等待整合者認領'}</p>
   {t.attention.length?<span className="attention">{t.attention.length} 個需關注</span>:null}
  </a>)}{!items.length?<p className="muted phase-empty">目前無任務</p>:null}</section>;
 })}</div></div>;
}
