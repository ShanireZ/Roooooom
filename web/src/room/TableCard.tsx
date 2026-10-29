import {forwardRef,type KeyboardEvent} from 'react';
import {useI18n} from '../i18n/I18nProvider';
import {ProgressBar} from '../ui/ProgressBar';
import {Stamp} from '../ui/Stamp';
import {routePath} from '../shell/routes';
import type {Table} from './floorModel';
import {Seat,useSeatLabels} from './Seat';
import {useArrivals} from '../ui/motion';

const stampFor={decision:'formal',acceptance:'acceptance',blocker:'blocker'} as const;
type Props={table:Table;itemRef:(el:HTMLLIElement|null)=>void;projectId:string;compact:boolean;dimmed:boolean;tabIndex:number;focusedMember?:string;onSelectMember:(id:string)=>void;onFocus:()=>void;onKeyDown:(e:KeyboardEvent<HTMLAnchorElement>)=>void};

export const TableCard=forwardRef<HTMLAnchorElement,Props>(function TableCard({table,itemRef,projectId,compact,dimmed,tabIndex,focusedMember,onSelectMember,onFocus,onKeyDown},ref){
 const {t,locale}=useI18n(),labels=useSeatLabels();
 // Flags and seats that appear after the card mounted land with a short motion.
 const newFlag=useArrivals(table.flags),newSeat=useArrivals(table.seats.map(s=>s.participantId));
 const phase=t(('phase.'+table.phase) as 'phase.active');
 const flags=table.flags.map(f=>t(('flag.'+f) as 'flag.decision'));
 // The accessible name carries goal, phase, owner flags and responsible seats.
 const name=new Intl.ListFormat(locale,{type:'unit',style:'short'}).format([table.goal,phase,...flags,...table.seats.map(s=>labels.role(s)+': '+labels.name(s)+', '+labels.sync(s))]);
 return <li ref={itemRef} className={'table-item'+(compact?' compact':'')+(table.flags.length?' flagged':'')} data-dimmed={String(dimmed)}>
  <a ref={ref} className={'table'+(compact?' compact':'')} href={routePath({name:'room',projectId,drawer:{kind:'task',id:table.taskId,tab:'overview'}})} data-local
   title={table.goal} aria-label={name} tabIndex={tabIndex} onFocus={onFocus} onKeyDown={onKeyDown}>
   {table.flags.length?<span className="flags">{table.flags.map(f=><Stamp key={f} kind={stampFor[f]} className={newFlag(f)?'arrive':undefined}>{t(('flag.'+f) as 'flag.decision')}</Stamp>)}</span>:null}
   <span className="goal">{table.goal}</span>
   <span className="table-phase muted">{phase}</span>
   <ProgressBar segments={table.segments}/>
   {!compact&&table.waitingForAgent?<span className="muted">{t('table.waitingAgent')}</span>:null}
   {!compact&&table.waitingOnDependencies?<span className="muted">{t('table.waitingDependencies')}</span>:null}
  </a>
  {table.seats.length?<div className="seats">{table.seats.map(s=><Seat key={s.participantId} seat={s} compact={compact} pressed={focusedMember===s.participantId} onSelect={onSelectMember} entering={newSeat(s.participantId)}/>)}</div>:null}
 </li>;
});
