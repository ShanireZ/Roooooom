import {useState,type ReactNode} from 'react';
import {useI18n} from '../i18n/I18nProvider';

// `lazy` mounts the content only while open, so a form inside binds the record
// version at the moment the owner opens it.
export function Fold({summary,open,lazy=false,children}:{summary:ReactNode;open?:boolean;lazy?:boolean;children:ReactNode}){
 const [shown,setShown]=useState(!!open);
 return <details className="fold" open={open} onToggle={e=>setShown(e.currentTarget.open)}><summary>{summary}</summary>{!lazy||shown?children:null}</details>;
}
// Identifiers, versions, epochs and heads live one layer down, never on the surface.
export function TechnicalDetails({children}:{children:ReactNode}){
 const {t}=useI18n();
 return <details className="fold"><summary>{t('fold.technical')}</summary><div className="tech">{children}</div></details>;
}
