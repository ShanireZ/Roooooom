export type Locale='en'|'zh-CN'|'zh-TW'|'ja';
export const LOCALES:readonly Locale[]=['en','zh-CN','zh-TW','ja'];

// Same mapping as the Go service (internal/i18n): Hant or TW/HK/MO regions are
// Traditional Chinese, any other zh is Simplified Chinese.
export function parseLocale(tag:string):Locale|undefined{
 const parts=tag.trim().toLowerCase().replaceAll('_','-').split('-');
 if(parts[0]==='en')return 'en';
 if(parts[0]==='ja')return 'ja';
 if(parts[0]!=='zh')return undefined;
 if(parts.includes('hans'))return 'zh-CN';
 if(parts.includes('hant')||parts.some(p=>p==='tw'||p==='hk'||p==='mo'))return 'zh-TW';
 return 'zh-CN';
}
export function detectLocale(languages:readonly string[]):Locale{
 for(const tag of languages){const locale=parseLocale(tag);if(locale)return locale}
 return 'en';
}
// The owner language (stored on the server) wins over browser detection.
export function resolveLocale(owner:string,languages:readonly string[]):Locale{
 return (LOCALES as readonly string[]).includes(owner)?owner as Locale:detectLocale(languages);
}
