import {useState} from 'react';
import type {Schema} from '../api/client';
import {useResource} from '../api/resource';
import {useI18n} from '../i18n/I18nProvider';
import {Feedback} from '../ui/Feedback';
import {joinInstructions,type ConnectionInfo} from './joinText';

export function JoinInstructions({project}:{project:Schema['ProjectView']}){
 const {t}=useI18n(),[copied,setCopied]=useState(false),[failed,setFailed]=useState(false);
 const info=useResource<ConnectionInfo>('/api/v1/setup/connection');
 const text=info.data?joinInstructions(t,{project,info:info.data}):'';
 return <section className="join">
  <p className="label">{t('setup.next')}</p>
  <pre className="join-instructions" data-testid="join-instructions">{text||t('setup.preparing')}</pre>
  <p className="muted">{t('door.notice')}</p>
  <button type="button" disabled={!text} onClick={()=>void navigator.clipboard.writeText(text).then(()=>setCopied(true)).catch(()=>setFailed(true))}>{copied?t('door.copied'):t('door.copy')}</button>
  {failed?<p className="muted">{t('door.copyFallback')}</p>:null}
  <Feedback error={info.error}/>
 </section>;
}
