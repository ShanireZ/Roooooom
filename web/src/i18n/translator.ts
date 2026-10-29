import {en,type MessageKey} from './catalog/en';
import {zhCN} from './catalog/zh-CN';
import {zhTW} from './catalog/zh-TW';
import {ja} from './catalog/ja';
import {LOCALES,type Locale} from './locale';
export {LOCALES,type Locale,type MessageKey};

export const catalogs:Record<Locale,Record<MessageKey,string>>={en,'zh-CN':zhCN,'zh-TW':zhTW,ja};
export type Params=Record<string,string|number>;
// A plural base is a key prefix whose `.one` and `.other` forms both exist.
export type PluralBase={[K in MessageKey]:K extends `${infer B}.one`?(`${B}.other` extends MessageKey?B:never):never}[MessageKey];
export type Translator={locale:Locale;t(key:MessageKey,params?:Params):string;tn(base:PluralBase,count:number,params?:Params):string};

export function hasMessage(key:string):key is MessageKey{return Object.hasOwn(en,key)}
const interpolate=(text:string,params:Params={})=>text.replace(/\{(\w+)\}/g,(whole,name:string)=>name in params?String(params[name]):whole);
export function createTranslator(locale:Locale):Translator{
 const rules=new Intl.PluralRules(locale);
 const t=(key:MessageKey,params?:Params)=>{
  const text=(catalogs[locale] as Record<string,string|undefined>)[key]??(en as Record<string,string|undefined>)[key];
  return text===undefined?'[missing:'+key+']':interpolate(text,params);
 };
 return {locale,t,tn:(base,count,params)=>t((base+(rules.select(count)==='one'?'.one':'.other')) as MessageKey,{count,...params})};
}
