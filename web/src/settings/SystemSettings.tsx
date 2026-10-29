import {useState} from 'react';
import {command} from '../api/client';
import {useWorkspaceEvents} from '../api/events';
import {useResource} from '../api/resource';
import {useOwner} from '../App';
import {useI18n} from '../i18n/I18nProvider';
import {LOCALES,type Locale} from '../i18n/locale';
import {applyTheme,readTheme,type ThemePreference} from '../theme/theme';
import {Feedback} from '../ui/Feedback';
import {Select} from '../ui/Field';
import {useAction} from '../ui/useAction';

type Metrics={storage:{databaseBytes:number;hardBytes:number;warning:boolean};counters:{commands:number;rejected:number;commandP95Millis:number}};
type Build={metadata?:{built?:boolean;version?:string;commit?:string;dirty?:boolean;sourceFingerprint?:string};goVersion?:string;goos?:string;goarch?:string};
const mib=(bytes:number,digits:number)=>(bytes/1048576).toFixed(digits);

// Machine-wide preferences and maintenance. Room-specific settings live in the room drawer.
export function SystemSettings(){
 const {t,ownerLanguage,setOwnerLanguage}=useI18n(),events=useWorkspaceEvents(),owner=useOwner();
 const language=useAction(),backup=useAction(),[theme,setTheme]=useState<ThemePreference>(readTheme()),[path,setPath]=useState('');
 const metrics=useResource<Metrics>('/api/v1/queries/metrics');
 const build=(owner?.build??{}) as Build,meta=build.metadata??{};
 const storage=metrics.data?.storage,counters=metrics.data?.counters;
 const run=(kind:'backup.create'|'backup.export',notice:string)=>void backup.run(async()=>{const r=await command<{path:string}>(kind,{});setPath(r.result.path);backup.setNotice(notice)});
 return <div className="card settings-page">
  <h1>{t('system.title')}</h1>
  <section aria-labelledby="system-language"><h2 id="system-language">{t('system.language')}</h2>
   <Select label={t('system.languageLabel')} value={ownerLanguage} disabled={!events.connected||language.busy} onChange={e=>{const value=e.target.value as Locale|'';void language.run(()=>setOwnerLanguage(value))}}>
    <option value="">{t('system.followBrowser')}</option>
    {LOCALES.map(l=><option key={l} value={l} lang={l}>{t(('language.'+l) as 'language.en')}</option>)}
   </Select>
   <p className="muted">{t('system.languageHint')}</p>
   <Feedback error={language.error}/>
  </section>
  <section aria-labelledby="system-theme"><h2 id="system-theme">{t('system.appearance')}</h2>
   <Select label={t('system.theme')} value={theme} onChange={e=>{const value=e.target.value as ThemePreference;applyTheme(value);setTheme(value)}}>
    <option value="system">{t('system.themeSystem')}</option><option value="light">{t('system.themeLight')}</option><option value="dark">{t('system.themeDark')}</option>
   </Select>
   <p className="muted">{t('system.themeHint')}</p>
  </section>
  <section aria-labelledby="system-backup"><h2 id="system-backup">{t('system.backup')}</h2>
   <p className="muted">{t('system.backupHelp')}</p>
   <div className="actions">
    <button type="button" disabled={!events.connected||backup.busy} onClick={()=>run('backup.create',t('system.snapshotDone'))}>{t('system.snapshot')}</button>
    <button type="button" className="btn-ghost" disabled={!events.connected||backup.busy} onClick={()=>run('backup.export',t('system.exportDone'))}>{t('system.export')}</button>
   </div>
   {path?<p className="private-path" data-testid="private-path"><span className="mono">{path}</span><br/><span className="muted">{t('system.privatePath')}</span></p>:null}
   <Feedback error={backup.error} notice={backup.notice}/>
  </section>
  <section aria-labelledby="system-storage"><h2 id="system-storage">{t('system.storage')}</h2>
   {storage?<p data-testid="storage" className={'storage'+(storage.warning?' warning':'')}>
    <meter min={0} max={storage.hardBytes} high={storage.hardBytes*0.75} value={storage.databaseBytes} aria-label={t('system.storage')}/>
    <span>{t('system.storageUse',{used:mib(storage.databaseBytes,1),cap:mib(storage.hardBytes,0)})}</span>
    {storage.warning?<strong>{t('system.storageWarning')}</strong>:null}
   </p>:null}
   {counters?<p className="muted mono">{t('system.counters',{commands:counters.commands,rejected:counters.rejected,p95:counters.commandP95Millis.toFixed(1)})}</p>:null}
   <p className="muted">{t('system.diskNote')}</p>
   <Feedback error={metrics.error}/>
  </section>
  <section aria-labelledby="system-restore"><h2 id="system-restore">{t('system.restore')}</h2>
   <p>{t('system.restoreHelp')}</p>
  </section>
  <section aria-labelledby="system-about"><h2 id="system-about">{t('system.about')}</h2>
   <dl className="about">
    <dt>{t('system.version')}</dt><dd className="mono">{meta.built?meta.version:t('system.devBuild')}</dd>
    {meta.sourceFingerprint?<><dt>{t('system.fingerprint')}</dt><dd className="mono">{meta.sourceFingerprint}</dd></>:null}
    {meta.commit?<><dt>{t('system.commit')}</dt><dd className="mono">{meta.commit}{meta.dirty?' · '+t('system.dirty'):''}</dd></>:null}
    <dt>{t('system.runtime')}</dt><dd className="mono">{[build.goVersion,build.goos&&build.goarch?build.goos+'/'+build.goarch:''].filter(Boolean).join(' · ')}</dd>
   </dl>
  </section>
 </div>;
}
