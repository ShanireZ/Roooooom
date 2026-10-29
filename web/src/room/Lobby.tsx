import {useI18n} from '../i18n/I18nProvider';
import {routePath} from '../shell/routes';
import type {Seat as SeatModel} from './floorModel';
import {Seat} from './Seat';

// Members without open responsibility wait in the lobby.
export function Lobby({members,anyMembers,projectId,focusedMember,onSelectMember}:{members:SeatModel[];anyMembers:boolean;projectId:string;focusedMember?:string;onSelectMember:(id:string)=>void}){
 const {t}=useI18n();
 return <section className="lobby">
  <h2 className="label">{t('room.lobby')}</h2>
  {!anyMembers?<p className="muted">{t('room.noAgents')} <a href={routePath({name:'room',projectId,drawer:{kind:'settings',section:'door'}})} data-local>{t('room.copyInstructions')}</a></p>
   :members.length?<ul className="lobby-list" aria-label={t('room.lobby')}>{members.map(s=><li key={s.participantId}><Seat seat={s} pressed={focusedMember===s.participantId} onSelect={onSelectMember}/></li>)}</ul>
   :<p className="muted">{t('room.lobbyEmpty')}</p>}
 </section>;
}
