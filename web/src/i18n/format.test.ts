import {describe,expect,it} from 'vitest';
import {formatDateTime,formatList,formatRelative} from './format';

const now=new Date('2026-10-29T12:00:00Z');

describe('formatRelative',()=>{
 it('describes recent times in the locale',()=>{
  expect(formatRelative('2026-10-29T11:58:00Z',now,'en')).toBe('2 minutes ago');
  expect(formatRelative('2026-10-29T09:00:00Z',now,'en')).toBe('3 hours ago');
  expect(formatRelative('2026-10-29T11:44:00Z',now,'ja')).toContain('16');
 });
 it('returns nothing for the never-observed sentinel',()=>{
  expect(formatRelative('0001-01-01T00:00:00Z',now,'en')).toBe('');
 });
});

describe('formatDateTime and formatList',()=>{
 it('format with Intl',()=>{
  expect(formatDateTime('2026-10-29T12:00:00Z','en')).toMatch(/2026/);
  expect(formatList(['Codex','Claude Code'],'en')).toBe('Codex and Claude Code');
 });
});
