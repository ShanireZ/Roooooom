import {useEffect,useLayoutEffect,useRef,useState,type KeyboardEvent} from 'react';
import {useI18n} from '../i18n/I18nProvider';
import {IconPlus} from '../ui/icons';
import {DURATION,EASE_OUT,reducedMotion,takeGhost} from '../ui/motion';
import type {Table} from './floorModel';
import {TableCard} from './TableCard';

type Props={tables:Table[];projectId:string;compact:boolean;focusedMember?:string;onSelectMember:(id:string)=>void;onAsk:()=>void};
// Tables in priority order with a roving tabindex. While a table has focus or the
// pointer is over the floor, the order is held so nothing jumps under the user.
export function Floor({tables,projectId,compact,focusedMember,onSelectMember,onAsk}:Props){
 const {t}=useI18n();
 const list=useRef<HTMLUListElement>(null),links=useRef<(HTMLAnchorElement|null)[]>([]);
 const hold=useRef({pointer:false,focus:false}),latest=useRef(tables);
 const held=()=>hold.current.pointer||hold.current.focus;
 const [order,setOrder]=useState(()=>tables.map(x=>x.taskId));
 const [active,setActive]=useState(0);
 latest.current=tables;
 useEffect(()=>{
  setOrder(previous=>held()
   ?[...previous.filter(id=>tables.some(x=>x.taskId===id)),...tables.filter(x=>!previous.includes(x.taskId)).map(x=>x.taskId)]
   :tables.map(x=>x.taskId));
 },[tables]);
 const release=()=>{if(!held())setOrder(latest.current.map(x=>x.taskId))};
 const byId=new Map(tables.map(x=>[x.taskId,x]));
 const shown=order.map(id=>byId.get(id)).filter((x):x is Table=>!!x);
 // FLIP: a table that changed place slides from where it was; a new table grows out of
 // the door note it was converted from, or fades in. The first render never animates.
 const items=useRef(new Map<string,HTMLLIElement>()),places=useRef(new Map<string,{x:number;y:number}>()),width=useRef(0),settled=useRef(false);
 useLayoutEffect(()=>{
  const host=list.current;
  if(!host)return;
  // A resize or drawer reflow is not a reorder: only animate within an unchanged width.
  const origin=host.getBoundingClientRect(),next=new Map<string,{x:number;y:number}>(),still=reducedMotion()||Math.abs(origin.width-width.current)>1;
  for(const table of shown){
   const el=items.current.get(table.taskId);
   if(!el)continue;
   const box=el.getBoundingClientRect(),place={x:box.left-origin.left,y:box.top-origin.top};
   next.set(table.taskId,place);
   if(still||!settled.current)continue;
   const before=places.current.get(table.taskId);
   if(before){
    const dx=before.x-place.x,dy=before.y-place.y;
    if(Math.abs(dx)>1||Math.abs(dy)>1)el.animate([{translate:dx+'px '+dy+'px'},{translate:'0 0'}],{duration:DURATION.shape,easing:EASE_OUT});
    continue;
   }
   const source=table.sourceRequestId,from=source?takeGhost(source)??document.querySelector('[data-request-id="'+CSS.escape(source)+'"]')?.getBoundingClientRect():undefined;
   if(from)el.animate([{transformOrigin:'0 0',translate:(from.left-box.left)+'px '+(from.top-box.top)+'px',scale:(from.width/box.width)+' '+(from.height/box.height),opacity:.7},{transformOrigin:'0 0',translate:'0 0',scale:'1 1',opacity:1}],{id:'morph',duration:DURATION.shape,easing:EASE_OUT});
   else el.animate([{opacity:0,scale:.96},{opacity:1,scale:1}],{duration:DURATION.base,easing:EASE_OUT});
  }
  places.current=next;width.current=origin.width;settled.current=true;
 });
 const move=(e:KeyboardEvent<HTMLAnchorElement>,index:number)=>{
  const next={ArrowRight:index+1,ArrowDown:index+1,ArrowLeft:index-1,ArrowUp:index-1,Home:0,End:shown.length-1}[e.key];
  if(next===undefined)return;
  e.preventDefault();
  const target=Math.max(0,Math.min(shown.length-1,next));
  setActive(target);links.current[target]?.focus();
 };
 return <ul ref={list} className={'tables'+(compact?' compact':'')} aria-label={t('room.floor')}
  onFocus={()=>{hold.current.focus=true}} onBlur={e=>{if(!e.currentTarget.contains(e.relatedTarget as Node|null)){hold.current.focus=false;release()}}}
  onMouseEnter={()=>{hold.current.pointer=true}} onMouseLeave={()=>{hold.current.pointer=false;release()}}>
  {shown.map((table,i)=><TableCard key={table.taskId} ref={el=>{links.current[i]=el}} itemRef={el=>{if(el)items.current.set(table.taskId,el);else items.current.delete(table.taskId)}} table={table} projectId={projectId} compact={compact}
   dimmed={!!focusedMember&&!table.seats.some(s=>s.participantId===focusedMember)} tabIndex={i===Math.min(active,shown.length-1)?0:-1}
   focusedMember={focusedMember} onSelectMember={onSelectMember} onFocus={()=>setActive(i)} onKeyDown={e=>move(e,i)}/>)}
  <li className="table-item add"><button type="button" className="table table-add" onClick={onAsk}><IconPlus/>{t('room.ask')}</button></li>
 </ul>;
}
