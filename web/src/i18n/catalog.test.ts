import {describe,expect,it} from 'vitest';
import {catalogs,createTranslator,LOCALES} from './translator';

const placeholders=(text:string)=>[...text.matchAll(/\{(\w+)\}/g)].map(m=>m[1]).sort();

describe('catalogs',()=>{
 it('have identical keys in every locale',()=>{
  const keys=Object.keys(catalogs.en).sort();
  expect(keys.length).toBeGreaterThan(0);
  for(const locale of LOCALES)expect(Object.keys(catalogs[locale]).sort(),locale).toEqual(keys);
 });
 it('use the same placeholders as English for every key',()=>{
  for(const locale of LOCALES)for(const [key,text] of Object.entries(catalogs.en)){
   expect(placeholders(catalogs[locale][key as keyof typeof catalogs.en]),locale+' '+key).toEqual(placeholders(text));
  }
 });
 it('have no empty messages',()=>{
  for(const locale of LOCALES)for(const [key,text] of Object.entries(catalogs[locale]))expect(text.trim(),locale+' '+key).not.toBe('');
 });
});

describe('translator',()=>{
 it('interpolates named placeholders',()=>{
  expect(createTranslator('en').t('time.lastSynced',{time:'2 minutes ago'})).toBe('Last synced 2 minutes ago');
 });
 it('chooses plural forms with Intl.PluralRules',()=>{
  const en=createTranslator('en');
  expect(en.tn('count.agents',1)).toBe('1 agent');
  expect(en.tn('count.agents',3)).toBe('3 agents');
  expect(createTranslator('ja').tn('count.agents',3)).toContain('3');
 });
 it('marks keys missing at runtime',()=>{
  expect(createTranslator('en').t('no.such.key' as never)).toBe('[missing:no.such.key]');
 });
});
