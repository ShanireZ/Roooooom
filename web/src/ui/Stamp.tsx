import type {ReactNode} from 'react';
import {useI18n} from '../i18n/I18nProvider';

export type StampKind='formal'|'acceptance'|'blocker'|'offline'|'danger';
const glyph:Record<StampKind,string>={formal:'⚖',acceptance:'✓',blocker:'⚠',offline:'↺',danger:''};
// Stamps always carry text; color alone never signals meaning.
export function Stamp({kind,children}:{kind:StampKind;children?:ReactNode}){
 const {t}=useI18n();
 const label=children??(kind==='danger'?null:t(('stamp.'+kind) as 'stamp.formal'));
 return <span className={'stamp stamp-'+kind}>{glyph[kind]?<span aria-hidden="true">{glyph[kind]}</span>:null}{label}</span>;
}
