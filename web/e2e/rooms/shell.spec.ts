import {test,expect} from '../fixture';
import {ownerCommand,ownerRead,seedProject} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

test('room tabs show per-room owner counts and the global Needs-you count',async({page,service,activate})=>{
 await activate();
 const first=await seedProject(page,service);
 const second=await ownerCommand(page,service,'project.create',{name:'Second room',repositories:[{name:'second',reference:'synthetic://second'}]});
 await ownerCommand(page,service,'task.create',{projectId:first.id,goal:'Risky change',scope:['fixture only'],nonScope:[],authorityRefs:['synthetic://owner'],risk:'high',acceptancePolicy:{mode:'integrator',conditions:['tests pass']},collaboratorLimit:0});
 const [question]=await ownerRead(page,service,'decisions',undefined,first.id);
 await page.goto(service.origin+'/rooms/'+first.id);
 const tabs=page.getByRole('navigation',{name:m('shell.rooms')});
 await expect(tabs.getByRole('link',{name:/Protocol rehearsal/})).toContainText('1');
 await expect(tabs.getByRole('link',{name:/Second room/})).not.toContainText('1');
 await expect(tabs.getByRole('link',{name:/Protocol rehearsal/})).toHaveAttribute('aria-current','page');
 const needs=page.getByRole('button',{name:new RegExp(m('shell.needsYou'))});
 await expect(needs).toContainText('1');
 await needs.click();
 await expect(needs).toHaveAttribute('aria-expanded','true');
 const list=page.getByRole('list',{name:m('shell.needsYou')});
 await expect(list.getByText('Protocol rehearsal')).toBeVisible();
 await list.getByRole('link').first().click();
 await expect(page).toHaveURL(service.origin+'/rooms/'+first.id+'/decisions/'+question.id);
 await page.getByRole('link',{name:/Second room/}).click();
 await expect(page).toHaveURL(service.origin+'/rooms/'+second.id);
});

test('read-only banner shows last sync and offers reconnect',async({page,service,activate})=>{
 await activate();
 const project=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+project.id);
 await expect(page.getByRole('heading',{name:'Protocol rehearsal'})).toBeVisible();
 await service.stop();
 const banner=page.getByRole('status').filter({hasText:m('shell.readOnly')});
 await expect(banner).toBeVisible({timeout:15000});
 await expect(banner.getByRole('button',{name:m('shell.reconnect')})).toBeVisible();
});

test('an unknown room path shows not-found inside the shell',async({page,service,activate})=>{
 await activate();
 await page.goto(service.origin+'/rooms/missing-room');
 await expect(page.getByText(m('room.notFound'))).toBeVisible();
 await expect(page.getByRole('navigation',{name:m('shell.rooms')})).toBeVisible();
});
