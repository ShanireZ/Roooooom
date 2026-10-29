import {useRef,type ReactNode} from 'react';
import {useInk} from './motion';

// A tab strip whose indicator slides to the current tab (aria-current="page").
export function InkNav({className,label,current,children}:{className:string;label:string;current:string;children:ReactNode}){
 const nav=useRef<HTMLElement>(null);
 useInk(nav,[current]);
 return <nav ref={nav} className={className} aria-label={label}>{children}<span className="tab-ink" aria-hidden="true"/></nav>;
}
