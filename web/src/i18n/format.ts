import type {Locale} from './locale';

// The Go service reports never-observed times as the zero time.
const never=(iso:string)=>iso.startsWith('0001-');
const units:[Intl.RelativeTimeFormatUnit,number][]=[['day',86400],['hour',3600],['minute',60],['second',1]];
export function formatRelative(iso:string,now:Date,locale:Locale):string{
 if(!iso||never(iso))return '';
 const seconds=Math.round((new Date(iso).getTime()-now.getTime())/1000);
 const [unit,size]=units.find(([,size])=>Math.abs(seconds)>=size)??units[units.length-1];
 return new Intl.RelativeTimeFormat(locale,{numeric:'auto'}).format(Math.trunc(seconds/size),unit);
}
export function formatDateTime(iso:string,locale:Locale):string{
 return never(iso)?'':new Intl.DateTimeFormat(locale,{dateStyle:'medium',timeStyle:'short'}).format(new Date(iso));
}
export function formatList(items:string[],locale:Locale):string{
 return new Intl.ListFormat(locale,{style:'long',type:'conjunction'}).format(items);
}
