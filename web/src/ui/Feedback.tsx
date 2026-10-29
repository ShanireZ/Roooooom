import {useI18n} from '../i18n/I18nProvider';
import {hasMessage} from '../i18n/translator';

const code=(value:string)=>/^[a-z0-9-]+$/.test(value);
// Plain localized sentence first; the raw server code stays in a diagnostic fold.
export function Feedback({error,notice}:{error?:string;notice?:string}){
 const {t}=useI18n();
 if(!error&&!notice)return null;
 const key='error.'+error;
 const message=error?(hasMessage(key)?t(key):code(error)?t('error.unknown'):error):'';
 return <>
  {error?<p className="error" role="alert">{message}</p>:null}
  {error&&code(error)?<details className="fold"><summary>{t('feedback.diagnostics')}</summary><code className="mono">{error}</code></details>:null}
  {notice?<p className="success" role="status">{notice}</p>:null}
 </>;
}
