import {test,expect} from '../fixture';
import {ownerCommand,seedProject,seedTask} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

test('/projects, /decisions, /tasks/:id, /requests/:id, /setup, /setup/:id reach their new routes',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Legacy table');
 const request=await ownerCommand(page,service,'request.create',{projectId:p.id,text:'Legacy request'});
 const room=service.origin+'/rooms/'+p.id;
 await page.goto(service.origin+'/projects');await expect(page).toHaveURL(room);
 await page.goto(service.origin+'/decisions');await expect(page).toHaveURL(room);
 await expect(page.locator('#needs-you-panel')).toBeVisible();
 await page.goto(service.origin+'/tasks/'+task.id);await expect(page).toHaveURL(room+'/tasks/'+task.id);
 await expect(page.getByRole('dialog')).toContainText('Legacy table');
 await page.goto(service.origin+'/requests/'+request.id);await expect(page).toHaveURL(room+'/requests/'+request.id);
 await expect(page.getByRole('dialog')).toContainText('Legacy request');
 for(const path of ['/setup','/setup/new']){await page.goto(service.origin+path);await expect(page).toHaveURL(service.origin+'/rooms/new')}
 await page.goto(service.origin+'/setup/'+p.id);await expect(page).toHaveURL(room+'/settings');
 await page.goto(service.origin+'/settings');
 await expect(page.getByRole('heading',{name:m('system.title'),exact:true})).toBeVisible();
 expect((await page.request.get(service.origin+'/rooms/settings')).status()).toBe(404);
});

test('an unknown task id shows not-found inside the shell',async({page,service,activate})=>{
 await activate();await seedProject(page,service);
 await page.goto(service.origin+'/tasks/missing-task');
 await expect(page.getByText(m('task.notFound'))).toBeVisible();
 await expect(page.getByRole('navigation',{name:m('shell.rooms')})).toBeVisible();
});

test('/ with no rooms opens /rooms/new',async({page,service,activate})=>{
 await activate();
 await page.goto(service.origin+'/');
 await expect(page).toHaveURL(service.origin+'/rooms/new');
 await expect(page.getByRole('heading',{name:m('setup.title'),exact:true})).toBeVisible();
});

test('/ returns to the last visited room',async({page,service,activate})=>{
 await activate();
 await seedProject(page,service);
 const second=await ownerCommand(page,service,'project.create',{name:'Second room',repositories:[{name:'r',reference:'synthetic://second'}]});
 await page.goto(service.origin+'/rooms/'+second.id);
 await expect(page.getByRole('heading',{name:'Second room'})).toBeVisible();
 await page.goto(service.origin+'/');
 await expect(page).toHaveURL(service.origin+'/rooms/'+second.id);
});

test('a legacy link opened while disconnected says it opens when the connection returns',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Offline legacy');
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(page.locator('.conn-state.conn-online')).toBeVisible();
 await service.stop();
 await expect(page.getByRole('status').filter({hasText:m('shell.readOnly')})).toBeVisible({timeout:15000});
 await page.evaluate(id=>{history.pushState(null,'','/tasks/'+id);dispatchEvent(new PopStateEvent('popstate'))},task.id);
 await expect(page.getByText(m('shell.waitingLink'))).toBeVisible();
});
