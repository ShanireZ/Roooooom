import {useI18n} from '../i18n/I18nProvider';

export function Stale({stale,onReopen}:{stale:boolean;onReopen:()=>void}){
 const {t}=useI18n();
 return stale?<p className="notice" role="status">{t('form.stale')} <button type="button" className="btn-ghost" onClick={onReopen}>{t('form.reopen')}</button></p>:null;
}
