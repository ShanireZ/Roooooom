import {useI18n} from '../i18n/I18nProvider';
import {IconCabinet} from '../ui/icons';
import {routePath} from '../shell/routes';
import type {FloorModel} from './floorModel';

// Completed work is filed away; the archive drawer searches all of it.
export function Cabinet({cabinet,projectId}:{cabinet:FloorModel['cabinet'];projectId:string}){
 const {t,tn}=useI18n();
 return <a className="cabinet" href={routePath({name:'room',projectId,drawer:{kind:'archive'}})} data-local>
  <span className="label"><IconCabinet/> {t('room.cabinet')}</span>
  <strong className="cabinet-count">{tn('room.cabinetDone',cabinet.doneCount)}</strong>
  {cabinet.recent.length?<ul className="cabinet-recent" aria-label={t('room.cabinetRecent')}>{cabinet.recent.map(r=><li key={r.taskId} title={r.goal}>{r.goal}</li>)}</ul>:null}
 </a>;
}
