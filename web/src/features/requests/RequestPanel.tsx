import {useRef,useState} from 'react';
import {command,ApiError,type Schema} from '../../api/client';
import {useWorkspaceEvents} from '../../api/events';
import {useCollection} from '../../api/resource';
import {Area,Field,Select,Feedback,lines,useAction,memberName,type Project,type Participant} from '../shared';
import {navigate} from '../../App';
export type WorkRequest=Schema['WorkRequest'];
export const requestStates:Record<string,string>={waiting:'待接手',preparing:'整理中',converted:'已轉任務',cancelled:'已取消'};
export function RequestPanel({projects,projectId,onDone,onCancel}:{projects:Project[];projectId:string;onDone:()=>void;onCancel:()=>void}){
 const [project,setProject]=useState(projectId),[uncertain,setUncertain]=useState(false),[acceptanceConditions,setAcceptanceConditions]=useState('');const action=useAction(),events=useWorkspaceEvents();
 const submitted=useRef<Schema['RequestCreate']|undefined>(undefined);
 const members=useCollection<Participant>('/api/v1/queries/participants?projectId='+encodeURIComponent(project),!!project);
 return <section className="surface"><h2>留下想做的事</h2><p className="muted">agent 接手後會整理範圍與分工，有不明確之處再找你拍板。</p><form data-testid="request-form" onSubmit={e=>{e.preventDefault();const form=new FormData(e.currentTarget);setUncertain(true);void action.run(async()=>{
  try{
   if(!submitted.current){const conditions=lines(String(form.get('conditions')??'')),mode=form.get('mode')==='human'?'human':'integrator';submitted.current={projectId:project,text:String(form.get('text')),constraints:lines(String(form.get('constraints')??'')),...(form.get('limit')?{collaboratorLimit:Number(form.get('limit'))}:{}),...(form.get('integrator')?{designatedIntegratorId:String(form.get('integrator'))}:{}),...(conditions.length?{acceptancePolicy:{mode,conditions,ownerProtected:true}}:{})}}
   // Disabled optional fields are absent from FormData on retry. Preserve the
   // first complete payload so the client reuses its exact command identity.
   const r=await command<WorkRequest>('request.create',submitted.current);submitted.current=undefined;setUncertain(false);onDone();navigate('/requests/'+r.result.id)
  }catch(error){if(error instanceof ApiError&&error.status<500){submitted.current=undefined;setUncertain(false)}throw error}
 })}}><fieldset disabled={!events.connected||action.busy}>
 {projects.length>1?<Select label="所屬專案" value={project} onChange={e=>setProject(e.target.value)} disabled={uncertain} required>{projects.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</Select>:null}
 <Area label="想做什麼" name="text" required readOnly={uncertain} maxLength={8192}/>
 <details><summary>更多要求（選填）</summary><fieldset disabled={uncertain}><Area label="不能改動或需要保留的要求" name="constraints"/><Field label="額外協作者上限 N（選填）" name="limit" type="number" min={0} max={64}/><Select label="指定整合 agent（選填）" name="integrator"><option value="">由合資格 agent 接手</option>{members.data?.map(p=><option key={p.id} value={p.id}>{memberName(p,members.data??[])}</option>)}</Select><p className="muted">整理仍由合資格 agent 認領；此處只指定後續整合責任。</p><Area label="明確驗收條件（選填，每行一項）" name="conditions" value={acceptanceConditions} onChange={e=>setAcceptanceConditions(e.target.value)}/>{acceptanceConditions.trim()?<Select label="上述條件由誰驗收" name="mode"><option value="integrator">整合 agent</option><option value="human">由我驗收</option></Select>:null}</fieldset></details>
 {uncertain?<p className="notice">結果尚未確認，原內容已保留。恢復連線後可明確重試同一需求。</p>:null}
 <div className="actions"><button type="submit">{uncertain?'重試原需求':'送出需求'}</button><button className="secondary" type="button" onClick={onCancel}>關閉</button></div>
 </fieldset><Feedback error={action.error}/></form></section>;
}
