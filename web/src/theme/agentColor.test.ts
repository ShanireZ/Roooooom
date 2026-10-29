import {describe,expect,it} from 'vitest';
import {agentColor} from './agentColor';

describe('agentColor',()=>{
 it('is stable for a participant id',()=>{
  expect(agentColor('participant-a')).toBe(agentColor('participant-a'));
 });
 it('spreads ids across the six agent colors',()=>{
  const buckets=[0,0,0,0,0,0];
  for(let i=0;i<600;i++)buckets[agentColor('p-'+i+'-'+(i*7919).toString(36))]++;
  for(const count of buckets){expect(count).toBeGreaterThanOrEqual(60);expect(count).toBeLessThanOrEqual(140)}
 });
});
