import type {ReactNode} from 'react';
import {useI18n} from '../i18n/I18nProvider';

export function Fold({summary,open,children}:{summary:ReactNode;open?:boolean;children:ReactNode}){
 return <details className="fold" open={open}><summary>{summary}</summary>{children}</details>;
}
// Identifiers, versions, epochs and heads live one layer down, never on the surface.
export function TechnicalDetails({children}:{children:ReactNode}){
 const {t}=useI18n();
 return <details className="fold"><summary>{t('fold.technical')}</summary><div className="tech">{children}</div></details>;
}
