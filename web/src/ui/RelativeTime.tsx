import {useEffect,useState} from 'react';
import {useI18n} from '../i18n/I18nProvider';
import {formatDateTime,formatRelative} from '../i18n/format';

// Re-renders every 30 seconds so "Last synced" text stays honest.
export function useNow(intervalMs=30000){
 const [now,setNow]=useState(()=>new Date());
 useEffect(()=>{const timer=setInterval(()=>setNow(new Date()),intervalMs);return()=>clearInterval(timer)},[intervalMs]);
 return now;
}
export function RelativeTime({iso}:{iso:string}){
 const {t,locale}=useI18n(),now=useNow();
 if(!iso||iso.startsWith('0001-'))return <span>{t('time.never')}</span>;
 return <time dateTime={iso} title={formatDateTime(iso,locale)}>{formatRelative(iso,now,locale)}</time>;
}
