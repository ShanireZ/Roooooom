import type {Schema} from '../../api/client';
import type {MessageKey,Params} from '../../i18n/translator';

export type NextStepInput={task:Schema['Task'];assignments:Schema['Assignment'][];needsYou:Schema['NeedsYouItem'][];names:Record<string,string>};
// One plain sentence about what happens next. It names responsibility, never live activity.
export function nextStep({task,assignments,needsYou,names}:NextStepInput):{key:MessageKey;params?:Params}{
 const mine=needsYou.filter(i=>i.taskId===task.id);
 const decision=mine.find(i=>i.kind==='decision');
 if(decision)return {key:'task.next.decision',params:{question:decision.summary}};
 if(mine.some(i=>i.kind==='acceptance'))return {key:'task.next.acceptance'};
 if(mine.some(i=>i.kind==='blocker'))return {key:'task.next.blocker'};
 if(task.phase==='done')return {key:'task.next.done'};
 if(task.phase==='cancelled')return {key:'task.next.cancelled'};
 if(!task.integratorId)return {key:'task.next.waitingAgent'};
 const name=names[task.integratorId]??task.integratorId.slice(0,8);
 if(task.phase==='integrating'||task.phase==='review'){
  const count=assignments.filter(a=>a.state==='submitted'||a.state==='accepted').length;
  return {key:'task.next.integrating',params:{name,count}};
 }
 if(!task.canStart)return {key:'task.next.dependencies'};
 return {key:'task.next.active',params:{name}};
}
