import {describe,it,expect,vi,afterEach} from 'vitest';
import {WorkspaceEvents} from './events';
describe('TestEventResetAndStaleCache',()=>{
 it('automatically recovers reads after a transient initial sync failure without a write',async()=>{
  let calls=0;
  class Source{addEventListener(){}close(){}}
  vi.stubGlobal('EventSource',Source);
  vi.stubGlobal('fetch',vi.fn(async(_path,options)=>{expect(options.method).toBe('GET');calls++;if(calls===1)throw new Error('offline');return {ok:true,json:async()=>({result:{events:[],cursor:'recovered',hasMore:false,bytes:0,serverTime:'recovered',connection:'online'}})}}));
  const events=new WorkspaceEvents();try{await events.start();await vi.waitFor(()=>expect(events.getSnapshot().lastSyncedAt).toBe('recovered'),{timeout:4500})}finally{events.stop()}
 });
 afterEach(()=>vi.unstubAllGlobals());
 it('marks disconnect read-only and requires a full reset for an old cursor',()=>{
  const events=new WorkspaceEvents();
  events.receive({events:[],cursor:'old',hasMore:false,bytes:0,serverTime:'2026-10-28T00:00:00Z',connection:'online'});
  expect(events.getSnapshot().connection).toBe('online');
  events.disconnect();expect(events.getSnapshot().connection).toBe('offline');
  expect(events.getSnapshot().lastSyncedAt).toBe('2026-10-28T00:00:00Z');
  events.invalidateCursor();expect(events.getSnapshot().resetRequired).toBe(true);
  expect(events.getSnapshot().connected).toBe(false);
 });
 it('does not enable writes just because an HTTP probe succeeds while SSE is disconnected',async()=>{
  let source:FakeSource|undefined;
  class FakeSource{
   onerror?:()=>void;
   constructor(){source=this}
   addEventListener(){} close(){}
  }
  vi.stubGlobal('EventSource',FakeSource);
  let calls=0;
  vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,json:async()=>({result:{events:[],cursor:'cursor',hasMore:false,bytes:0,serverTime:++calls===1?'initial':'probe',connection:'online'}})})));
  const events=new WorkspaceEvents();await events.start();
  source!.onerror!();
  await vi.waitFor(()=>expect(events.getSnapshot().lastSyncedAt).toBe('probe'));
  expect(events.getSnapshot().connected).toBe(false);
  expect(events.getSnapshot().connection).toBe('offline');
  events.stop();
 });
});
