import {afterEach,describe,expect,it} from 'vitest';
import {applyTheme,readTheme} from './theme';

const original={localStorage:globalThis.localStorage,document:globalThis.document};
afterEach(()=>{Object.assign(globalThis,original)});

describe('theme preference',()=>{
 it('applies and remembers the choice',()=>{
  const store=new Map<string,string>(),root={dataset:{} as Record<string,string>};
  Object.assign(globalThis,{localStorage:{getItem:(k:string)=>store.get(k)??null,setItem:(k:string,v:string)=>{store.set(k,v)}},document:{documentElement:root}});
  applyTheme('dark');
  expect(root.dataset.theme).toBe('dark');
  expect(readTheme()).toBe('dark');
 });
 it('survives unavailable storage',()=>{
  const root={dataset:{} as Record<string,string>};
  const broken=()=>{throw new Error('blocked')};
  Object.assign(globalThis,{localStorage:{getItem:broken,setItem:broken},document:{documentElement:root}});
  expect(()=>applyTheme('light')).not.toThrow();
  expect(root.dataset.theme).toBe('light');
  expect(readTheme()).toBe('system');
 });
 it('ignores unknown stored values',()=>{
  Object.assign(globalThis,{localStorage:{getItem:()=>'neon',setItem:()=>{}}});
  expect(readTheme()).toBe('system');
 });
});
