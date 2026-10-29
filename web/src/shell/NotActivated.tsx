import {useI18n} from '../i18n/I18nProvider';

// Shown until the one-time activation page has been used on this computer.
export function NotActivated({checking,offline,retry}:{checking:boolean;offline:boolean;retry:()=>void}){
 const {t}=useI18n();
 return <section className="card not-activated">
  <p className="label">{t('app.name')}</p>
  <h1>{checking?t('notActivated.checking'):t('notActivated.title')}</h1>
  <p>{t('notActivated.body')}</p>
  <p className="muted">{t('notActivated.note')}</p>
  {offline?<p className="error" role="alert">{t('notActivated.offline')}</p>:null}
  <button type="button" onClick={retry} disabled={checking}>{t('notActivated.retry')}</button>
 </section>;
}
