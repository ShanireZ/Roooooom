import {createContext,useContext,useEffect,useMemo,useState,type ReactNode} from 'react';
import {command} from '../api/client';
import {detectLocale,LOCALES,resolveLocale,type Locale} from './locale';
import {createTranslator,type Translator} from './translator';

// The cache only avoids a flash of the wrong language before the owner context
// loads; the server-stored owner language is the authority.
const CACHE='roooooom.locale';
function cachedLocale():Locale|undefined{
 try{const value=localStorage.getItem(CACHE);return value&&(LOCALES as readonly string[]).includes(value)?value as Locale:undefined}
 catch{return undefined}
}
export type I18nValue=Translator&{ownerLanguage:string;setOwnerLanguage(value:Locale|''):Promise<void>};
const I18n=createContext<I18nValue|undefined>(undefined);

export function I18nProvider({ownerLanguage,children}:{ownerLanguage?:string;children:ReactNode}){
 const [chosen,setChosen]=useState<string>();
 const owner=chosen??ownerLanguage;
 const locale=owner===undefined?cachedLocale()??detectLocale(navigator.languages):resolveLocale(owner,navigator.languages);
 const translator=useMemo(()=>createTranslator(locale),[locale]);
 useEffect(()=>{
  document.documentElement.lang=locale;
  try{localStorage.setItem(CACHE,locale)}catch{/* storage may be unavailable */}
 },[locale]);
 const value=useMemo<I18nValue>(()=>({...translator,ownerLanguage:owner??'',setOwnerLanguage:async language=>{
  // An explicit owner choice is a normal versioned write, never automatic.
  await command('owner.language',{language});
  setChosen(language);
 }}),[translator,owner]);
 return <I18n.Provider value={value}>{children}</I18n.Provider>;
}
export function useI18n():I18nValue{
 const value=useContext(I18n);
 if(!value)throw new Error('useI18n requires I18nProvider');
 return value;
}
