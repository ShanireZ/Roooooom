import type {ReactNode} from 'react';

// Stroke icons at the 1.5px rule weight; always decorative next to text.
function Icon({children}:{children:ReactNode}){
 return <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="square" aria-hidden="true">{children}</svg>;
}
export const IconDoor=()=><Icon><path d="M6 21V3h9l3 2v16"/><path d="M3 21h18"/><path d="M13 12h1"/></Icon>;
export const IconTable=()=><Icon><rect x="4" y="7" width="16" height="10"/><path d="M8 4v3M16 4v3M8 17v3M16 17v3"/></Icon>;
export const IconCabinet=()=><Icon><rect x="5" y="3" width="14" height="18"/><path d="M5 9h14M5 15h14M11 6h2M11 12h2M11 18h2"/></Icon>;
export const IconGear=()=><Icon><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1"/></Icon>;
export const IconClose=()=><Icon><path d="M6 6l12 12M18 6L6 18"/></Icon>;
export const IconPlus=()=><Icon><path d="M12 5v14M5 12h14"/></Icon>;
