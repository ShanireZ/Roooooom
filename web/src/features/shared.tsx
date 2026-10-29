import {useId,useRef,useState,type InputHTMLAttributes,type TextareaHTMLAttributes,type SelectHTMLAttributes,type ReactNode} from 'react';
import {ApiError,type Schema} from '../api/client';
export type Task=Schema['Task'];
export type Assignment=Schema['Assignment'];
export type Project=Schema['ProjectView'];
export type Participant=Schema['ParticipantView'];
export function memberName(p:Participant,roster:Participant[]){return p.name+(roster.filter(x=>x.name===p.name).length>1?' · '+p.id.slice(0,8):'')}
export const lines=(text:string)=>text.split(/\r?\n/).map(x=>x.trim()).filter(Boolean);
export const phases:Record<string,string>={ready:'待接手',active:'進行中',integrating:'整合中',review:'待驗收',done:'已完成',cancelled:'已取消'};
export const states:Record<string,string>={open:'開放認領',claimed:'已認領',active:'進行中',submitted:'已提交',accepted:'已接收',cancelled:'已取消'};
export function versions(task:Task){return {expectedVersion:task.version,integratorEpoch:task.integratorEpoch,specVersion:task.specVersion,acceptancePolicyVersion:task.acceptancePolicyVersion,decisionVersions:task.decisionVersions}}
export function when(value:string){return value.startsWith('0001-')?'尚未觀測':new Date(value).toLocaleString('zh-TW')}
export function useAction(){
 const busyRef=useRef(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
 const run=async<T,>(work:()=>Promise<T>):Promise<T|undefined>=>{
  if(busyRef.current)return;busyRef.current=true;setBusy(true);setError('');setNotice('');
  try{return await work()}catch(error){setError(error instanceof ApiError?error.code:'操作未確認，請重讀狀態後再處理。')}
  finally{busyRef.current=false;setBusy(false)}
 };
 return {busy,error,notice,setNotice,run};
}
const plainErrors:Record<string,string>={offline:'服務暫時無法連線。保留目前內容，恢復後先核對狀態再明確重試。',invalid:'請檢查輸入內容與必要條件，再送出一次。',forbidden:'這個操作需要相應權限，請確認目前身分。','github-metadata-unavailable':'目前無法讀取 GitHub。憑證已保留，可稍後再確認目的地。','github-metadata-denied-or-moved':'GitHub 未允許讀取此目的地，請檢查網址、PAT 範圍與 repository 權限。','github-repository-unavailable':'此 repository 無法用於 Issues 紀錄，請確認 Issues 已開放且未封存。','read-only-until-synchronized':'正在恢復讀取，完成同步後才能操作。'};
export function Feedback({error,notice}:{error?:string;notice?:string}){return <>{error?<><p className="error" role="alert">{plainErrors[error]??(/^[a-z0-9-]+$/.test(error)?'操作尚未完成，請重新讀取目前狀態後確認。':error)}</p>{/^[a-z0-9-]+$/.test(error)?<details><summary>診斷資訊</summary><code>{error}</code></details>:null}</>:null}{notice?<p className="success" role="status">{notice}</p>:null}</>}
export function Field({label,identifier=false,...props}:InputHTMLAttributes<HTMLInputElement>&{label:string;identifier?:boolean}){
 const id=useId();return <label htmlFor={id}>{label}<input id={id} name={props.name??id} {...(identifier?{autoCorrect:'off',autoCapitalize:'off',spellCheck:false}:{})} {...props}/></label>;
}
export function Area({label,...props}:TextareaHTMLAttributes<HTMLTextAreaElement>&{label:string}){
 const id=useId();return <div className="field"><label htmlFor={id}>{label}</label><textarea id={id} name={props.name??id} {...props}/></div>;
}
export function Select({label,children,...props}:SelectHTMLAttributes<HTMLSelectElement>&{label:string;children:ReactNode}){
 const id=useId();return <div className="field"><label htmlFor={id}>{label}</label><select id={id} name={props.name??id} {...props}>{children}</select></div>;
}
export function Toggle({label,...props}:InputHTMLAttributes<HTMLInputElement>&{label:string}){
 const id=useId();return <label className="choice" htmlFor={id}><input id={id} name={props.name??id} type="checkbox" {...props}/><span>{label}</span></label>;
}
export function List({items}:{items?:string[]}){return items?.length?<ul className="plain-list">{items.map((x,i)=><li key={i}>{x}</li>)}</ul>:<p className="muted">無</p>}
