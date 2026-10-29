import {describe,expect,it} from 'vitest';
import {detectLocale,resolveLocale} from './locale';

describe('detectLocale',()=>{
 it('maps browser tags to the four product locales',()=>{
  const cases:Record<string,string>={
   'zh-TW':'zh-TW','zh-HK':'zh-TW','zh-MO':'zh-TW','zh-Hant':'zh-TW','zh-Hant-TW':'zh-TW','ZH_tw':'zh-TW',
   'zh':'zh-CN','zh-CN':'zh-CN','zh-SG':'zh-CN','zh-Hans':'zh-CN','zh-Hans-HK':'zh-CN',
   'ja':'ja','ja-JP':'ja','en':'en','en-GB':'en',
  };
  for(const [tag,want] of Object.entries(cases))expect(detectLocale([tag]),tag).toBe(want);
 });
 it('skips unsupported tags and falls back to English',()=>{
  expect(detectLocale(['pt-BR','ja'])).toBe('ja');
  expect(detectLocale(['fr','de'])).toBe('en');
  expect(detectLocale([])).toBe('en');
 });
});

describe('resolveLocale',()=>{
 it('prefers the owner language over the browser',()=>{
  expect(resolveLocale('zh-CN',['ja'])).toBe('zh-CN');
 });
 it('follows the browser when the owner language is unset or invalid',()=>{
  expect(resolveLocale('',['ja-JP'])).toBe('ja');
  expect(resolveLocale('fr',['zh-TW'])).toBe('zh-TW');
 });
});
