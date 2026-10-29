import {useWorkspaceEvents} from '../api/events';
import {useI18n} from '../i18n/I18nProvider';
import {RelativeTime} from '../ui/RelativeTime';

// Top-bar connection state: online, re-synchronizing, or read-only.
export function ConnectionIndicator(){
 const {t}=useI18n(),events=useWorkspaceEvents();
 const label=events.connected?t('shell.online'):events.connection==='reconciling'?t('shell.reconciling'):t('shell.offline');
 return <span className={'connection connection-'+(events.connected?'online':events.connection)}>{label}</span>;
}
// While disconnected the floor stays visible from the in-memory snapshot and writes are disabled.
export function ConnectionBanner({retry,busy}:{retry:()=>void;busy:boolean}){
 const {t}=useI18n(),events=useWorkspaceEvents();
 if(events.resetRequired)return <div className="banner banner-alert" role="alert"><span>{t('shell.resetRequired')}</span><button type="button" onClick={retry} disabled={busy}>{t('shell.reconnect')}</button></div>;
 if(events.connected)return null;
 return <div className="banner" role="status">
  <strong>{t('shell.readOnly')}</strong>
  <span className="muted">{events.lastSyncedAt?<>{t('shell.lastSync')} <RelativeTime iso={events.lastSyncedAt}/></>:t('shell.loadingState')}</span>
  <button type="button" className="btn-ghost" onClick={retry} disabled={busy}>{t('shell.reconnect')}</button>
 </div>;
}
