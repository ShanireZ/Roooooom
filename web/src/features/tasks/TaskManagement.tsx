import {useRef,useState} from 'react';
import {command} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {Field,Feedback,useAction,versions,type Task} from '../shared';
export function TaskManagement({task,fresh,refresh}:{task:Task;fresh:boolean;refresh:()=>void}){
 const [open,setOpen]=useState(false),[notice,setNotice]=useState('');
 const done=(message:string)=>{setNotice(message);setOpen(false);refresh()};
 return <><details open={open} onToggle={e=>setOpen(e.currentTarget.open)}><summary>任務管理</summary>{open?<ManagementForms task={task} fresh={fresh} done={done}/>:null}</details><Feedback notice={notice}/></>;
}
function ManagementForms({task,fresh,done}:{task:Task;fresh:boolean;done:(message:string)=>void}){
 const base=useRef(versions(task)).current,action=useAction(),event=useWorkspaceEvents();
 const stale=base.expectedVersion!==task.version,disabled=!fresh||!event.connected||stale||action.busy;
 return <>{stale?<p className="notice">任務版本已變更，請關閉並重新開啟管理表單。</p>:null}
  <form action="/api/v1/commands/collaborator.limit" method="post" onSubmit={e=>{
   e.preventDefault();const form=new FormData(e.currentTarget);
   void action.run(async()=>{await command('collaborator.limit',{limit:Number(form.get('limit')),reason:String(form.get('reason'))},task.id,base);done('協作上限已更新。')});
  }}><fieldset disabled={disabled}><legend>調整協作名額</legend><Field label="新的額外協作者上限" type="number" name="limit" min={task.collaboratorCount} max={64} defaultValue={task.collaboratorLimit} required/><Field label="調整理由" name="reason" required/><button type="submit">更新協作上限</button></fieldset></form>
  {task.phase==='cancelled'?<p className="muted">已取消的任務保留歷史；新的工作可另登記任務。</p>:<form className="inset" action={'/api/v1/commands/'+(task.phase==='done'?'task.reopen':'task.cancel')} method="post" onSubmit={e=>{
   e.preventDefault();const reason=String(new FormData(e.currentTarget).get('reason'));
   void action.run(async()=>{await command(task.phase==='done'?'task.reopen':'task.cancel',{reason},task.id,base);done(task.phase==='done'?'任務已重新開啟審閱。':'任務已取消，歷史與責任紀錄保留。')});
  }}><fieldset disabled={disabled}><legend>{task.phase==='done'?'重新開啟審閱':'取消任務'}</legend><Field label={task.phase==='done'?'重新開啟理由':'取消任務理由'} name="reason" required/><button className={task.phase==='done'?'secondary':'danger'} type="submit">{task.phase==='done'?'重新開啟任務':'確認取消任務'}</button></fieldset></form>}
  <Feedback error={action.error}/>
 </>;
}
