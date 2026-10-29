import {useEffect,useRef,useState,type KeyboardEvent} from 'react';
import {useI18n} from '../i18n/I18nProvider';
import {IconPlus} from '../ui/icons';
import type {Table} from './floorModel';
import {TableCard} from './TableCard';

type Props={tables:Table[];projectId:string;compact:boolean;focusedMember?:string;onSelectMember:(id:string)=>void;onAsk:()=>void};
// Tables in priority order with a roving tabindex. While a table has focus or the
// pointer is over the floor, the order is held so nothing jumps under the user.
export function Floor({tables,projectId,compact,focusedMember,onSelectMember,onAsk}:Props){
 const {t}=useI18n();
 const list=useRef<HTMLUListElement>(null),links=useRef<(HTMLAnchorElement|null)[]>([]);
 const frozen=useRef(false),latest=useRef(tables);
 const [order,setOrder]=useState(()=>tables.map(x=>x.taskId));
 const [active,setActive]=useState(0);
 latest.current=tables;
 useEffect(()=>{
  setOrder(previous=>frozen.current
   ?[...previous.filter(id=>tables.some(x=>x.taskId===id)),...tables.filter(x=>!previous.includes(x.taskId)).map(x=>x.taskId)]
   :tables.map(x=>x.taskId));
 },[tables]);
 const thaw=()=>{frozen.current=false;setOrder(latest.current.map(x=>x.taskId))};
 const byId=new Map(tables.map(x=>[x.taskId,x]));
 const shown=order.map(id=>byId.get(id)).filter((x):x is Table=>!!x);
 const move=(e:KeyboardEvent<HTMLAnchorElement>,index:number)=>{
  const next={ArrowRight:index+1,ArrowDown:index+1,ArrowLeft:index-1,ArrowUp:index-1,Home:0,End:shown.length-1}[e.key];
  if(next===undefined)return;
  e.preventDefault();
  const target=Math.max(0,Math.min(shown.length-1,next));
  setActive(target);links.current[target]?.focus();
 };
 return <ul ref={list} className={'tables'+(compact?' compact':'')} aria-label={t('room.floor')}
  onFocus={()=>{frozen.current=true}} onBlur={e=>{if(!e.currentTarget.contains(e.relatedTarget as Node|null))thaw()}}
  onMouseEnter={()=>{frozen.current=true}} onMouseLeave={()=>{if(!list.current?.contains(document.activeElement))thaw()}}>
  {shown.map((table,i)=><TableCard key={table.taskId} ref={el=>{links.current[i]=el}} table={table} projectId={projectId} compact={compact}
   dimmed={!!focusedMember&&!table.seats.some(s=>s.participantId===focusedMember)} tabIndex={i===Math.min(active,shown.length-1)?0:-1}
   focusedMember={focusedMember} onSelectMember={onSelectMember} onFocus={()=>setActive(i)} onKeyDown={e=>move(e,i)}/>)}
  <li className="table-item"><button type="button" className="table table-add" onClick={onAsk}><IconPlus/>{t('room.ask')}</button></li>
 </ul>;
}
