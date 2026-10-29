import {test,expect} from '../fixture';
import {claimIntegrator,enroll,makeCandidate,ownerRead,seedProject,seedTask} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

const floorLink=(page:import('@playwright/test').Page,text:string)=>page.getByRole('list',{name:m('room.floor')}).getByRole('link',{name:new RegExp(text)});

test('drawer opens from a table, Escape closes it and focus returns',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const task=await seedTask(page,service,p,'integrator','Drawer goal');
 await page.goto(service.origin+'/rooms/'+p.id);
 await floorLink(page,'Drawer goal').click();
 const drawer=page.getByRole('dialog',{name:'Drawer goal'});
 await expect(drawer).toBeVisible();
 await expect(page).toHaveURL(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
 await page.keyboard.press('Escape');
 await expect(drawer).toBeHidden();
 await expect(page).toHaveURL(service.origin+'/rooms/'+p.id);
 await expect(floorLink(page,'Drawer goal')).toBeFocused();
});

test('browser Back closes the drawer',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 await seedTask(page,service,p,'integrator','Back goal');
 await page.goto(service.origin+'/rooms/'+p.id);
 await floorLink(page,'Back goal').click();
 await expect(page.getByRole('dialog',{name:'Back goal'})).toBeVisible();
 await page.goBack();
 await expect(page.getByRole('dialog')).toHaveCount(0);
});

test('overview shows the next step; technical details hold identifiers and versions',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const task=await seedTask(page,service,p,'integrator','Plain goal');
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
 const drawer=page.getByRole('dialog',{name:'Plain goal'});
 await expect(drawer).toContainText(m('task.next.waitingAgent'));
 const tech=drawer.getByText(m('fold.technical'));
 await expect(drawer.getByText(task.id)).toBeHidden();
 await tech.click();
 await expect(drawer.getByText(task.id,{exact:false})).toBeVisible();
 await expect(drawer).toContainText('epoch');
});

test('evidence summarizes deliveries and keeps exact heads in a fold',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const integrator=await enroll(page,service,p.id,'Codex');
 const task=await seedTask(page,service,p,'integrator','Evidence goal');
 await claimIntegrator(page,service,integrator,task.id);
 await makeCandidate(page,service,p,task.id,[integrator]);
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id+'/evidence');
 const drawer=page.getByRole('dialog',{name:'Evidence goal'});
 const delivery=drawer.getByRole('listitem').filter({hasText:'Codex'}).first();
 await expect(delivery).toBeVisible();
 await expect(drawer.getByText('1'.repeat(40))).toBeHidden();
 await delivery.getByText(m('fold.technical')).click();
 await expect(drawer.getByText('1'.repeat(40),{exact:false}).first()).toBeVisible();
});

test('discussion posts to the task thread',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const task=await seedTask(page,service,p,'integrator','Talk goal');
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id+'/discussion');
 const drawer=page.getByRole('dialog',{name:'Talk goal'});
 await drawer.getByLabel(m('composer.sayLabel')).fill('Task thread message');
 await drawer.getByRole('button',{name:m('composer.saySubmit')}).click();
 await expect(drawer.getByRole('list',{name:m('task.tab.discussion')})).toContainText('Task thread message');
 const thread=await ownerRead(page,service,'thread',task.id);
 expect(thread[0]).toMatchObject({taskId:task.id,threadId:'task',body:'Task thread message'});
});

test('the full long goal is readable in the drawer on a narrow screen',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const goal='Long goal '+'長文目錄與搜尋功能🙂'.repeat(180);
 const task=await seedTask(page,service,p,'integrator',goal);
 await page.setViewportSize({width:375,height:800});
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
 await expect(page.getByRole('heading',{level:2,name:goal})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
});
