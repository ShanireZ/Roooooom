import {useEffect,useLayoutEffect,useRef,useState,type RefObject} from 'react';

// Motion helpers for the refined blueprint (docs/ui-design.md §10.4). Nothing here
// delays state: motion only decorates a change that has already been rendered,
// and every helper stands still when the reader prefers reduced motion.
export const DURATION={quick:120,base:180,shape:280};
export const EASE_OUT='cubic-bezier(.2,.75,.2,1)';
export const reducedMotion=()=>typeof matchMedia==='function'&&matchMedia('(prefers-reduced-motion: reduce)').matches;

// Increases each time `value` changes after the first render. Used as a React key
// so a badge remounts and replays its pop only for real changes.
export function useBump(value:unknown):number{
 const previous=useRef(value),[count,setCount]=useState(0);
 useEffect(()=>{if(!Object.is(previous.current,value)){previous.current=value;setCount(n=>n+1)}},[value]);
 return count;
}

// Keys present on the first render are settled; later keys are arrivals that may animate in.
export function useArrivals(keys:readonly string[]):(key:string)=>boolean{
 const initial=useRef<Set<string>|null>(null);
 if(initial.current===null)initial.current=new Set(keys);
 const settled=initial.current;
 return key=>!settled.has(key);
}

// Places the nav's `.tab-ink` child under its aria-current="page" child.
export function useInk(nav:RefObject<HTMLElement|null>,deps:readonly unknown[]){
 useLayoutEffect(()=>{
  const el=nav.current;
  if(!el)return;
  const place=()=>{
   const ink=el.querySelector<HTMLElement>(':scope>.tab-ink'),current=el.querySelector<HTMLElement>(':scope>[aria-current=page]');
   if(!ink)return;
   ink.style.opacity=current?'1':'0';
   if(current){ink.style.left=current.offsetLeft+'px';ink.style.width=current.offsetWidth+'px'}
  };
  place();
  const observer=new ResizeObserver(place);
  observer.observe(el);
  return()=>observer.disconnect();
 // eslint-disable-next-line react-hooks/exhaustive-deps
 },deps);
}

// The last on-screen box of a door note that just left, so its new table can grow out of it.
const ghosts=new Map<string,{rect:DOMRect;at:number}>();
export function leaveGhost(key:string,rect:DOMRect){ghosts.set(key,{rect,at:performance.now()})}
export function takeGhost(key:string):DOMRect|undefined{
 const ghost=ghosts.get(key);
 ghosts.delete(key);
 return ghost&&performance.now()-ghost.at<3000?ghost.rect:undefined;
}
