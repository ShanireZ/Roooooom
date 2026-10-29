import {describe,expect,it} from 'vitest';
import {homeTarget} from './home';

const rooms=[{id:'a'},{id:'b'},{id:'c'}];
describe('homeTarget',()=>{
 it('prefers the last visited room while it still exists',()=>expect(homeTarget(rooms,[{projectId:'c'}],'b')).toBe('/rooms/b'));
 it('falls back to the first room with owner items',()=>expect(homeTarget(rooms,[{projectId:'c'}],'gone')).toBe('/rooms/c'));
 it('then to the first room by creation order',()=>expect(homeTarget(rooms,[],null)).toBe('/rooms/a'));
 it('opens room creation when there are no rooms',()=>expect(homeTarget([],[],'a')).toBe('/rooms/new'));
});
