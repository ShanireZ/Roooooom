import {describe,expect,it} from 'vitest';
import {parseRoute,routePath,type Route} from './routes';

describe('parseRoute',()=>{
 it('round-trips every canonical spec route',()=>{
  const rows:[string,Route][]=[
   ['/',{name:'home'}],
   ['/rooms/new',{name:'newRoom'}],
   ['/settings',{name:'settings'}],
   ['/rooms/p1',{name:'room',projectId:'p1'}],
   ['/rooms/p1/tasks/t1',{name:'room',projectId:'p1',drawer:{kind:'task',id:'t1',tab:'overview'}}],
   ['/rooms/p1/tasks/t1/work',{name:'room',projectId:'p1',drawer:{kind:'task',id:'t1',tab:'work'}}],
   ['/rooms/p1/tasks/t1/manage',{name:'room',projectId:'p1',drawer:{kind:'task',id:'t1',tab:'manage'}}],
   ['/rooms/p1/requests/r1',{name:'room',projectId:'p1',drawer:{kind:'request',id:'r1'}}],
   ['/rooms/p1/decisions/d1',{name:'room',projectId:'p1',drawer:{kind:'decision',id:'d1'}}],
   ['/rooms/p1/acceptance/t1',{name:'room',projectId:'p1',drawer:{kind:'acceptance',taskId:'t1'}}],
   ['/rooms/p1/archive',{name:'room',projectId:'p1',drawer:{kind:'archive'}}],
   ['/rooms/p1/settings',{name:'room',projectId:'p1',drawer:{kind:'settings',section:'door'}}],
   ['/rooms/p1/settings/members',{name:'room',projectId:'p1',drawer:{kind:'settings',section:'members'}}],
  ];
  for(const [path,route] of rows){
   expect(parseRoute(path),path).toEqual(route);
   expect(routePath(route),path).toBe(path);
  }
 });
 it('accepts explicit default segments',()=>{
  expect(parseRoute('/rooms/p1/tasks/t1/overview')).toEqual({name:'room',projectId:'p1',drawer:{kind:'task',id:'t1',tab:'overview'}});
  expect(parseRoute('/rooms/p1/settings/door')).toEqual({name:'room',projectId:'p1',drawer:{kind:'settings',section:'door'}});
 });
 it('maps legacy paths',()=>{
  expect(parseRoute('/projects')).toEqual({name:'legacy',to:'home'});
  expect(parseRoute('/decisions')).toEqual({name:'legacy',to:'needsYou'});
  expect(parseRoute('/setup')).toEqual({name:'legacy',to:'newRoom'});
  expect(parseRoute('/setup/new')).toEqual({name:'legacy',to:'newRoom'});
  expect(parseRoute('/setup/abc')).toEqual({name:'legacy',to:'roomDoor',projectId:'abc'});
  expect(parseRoute('/tasks/t9')).toEqual({name:'legacyTask',id:'t9'});
  expect(parseRoute('/requests/r9')).toEqual({name:'legacyRequest',id:'r9'});
 });
 it('rejects unknown or malformed paths',()=>{
  for(const path of ['/rooms/settings','/rooms/x/tasks/y/bogus','/rooms/x/nope','/rooms','/rooms/a b','/rooms/x/settings/other','/elsewhere','/rooms/'+'x'.repeat(129)])expect(parseRoute(path),path).toEqual({name:'notFound'});
 });
});
