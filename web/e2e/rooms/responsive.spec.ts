import {test,expect} from '../fixture';
import {claimIntegrator,enroll,ownerCommand,seedProject,seedTask} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

const isModal=(page:import('@playwright/test').Page)=>page.locator('dialog.drawer').evaluate(d=>d.matches(':modal'));

test('at 1280 px the drawer is non-modal and the floor stays clickable',async({page,service,activate})=>{
 await page.setViewportSize({width:1280,height:800});
 await activate();
 const p=await seedProject(page,service),a=await seedTask(page,service,p,'integrator','First wide table'),b=await seedTask(page,service,p,'integrator','Second wide table');
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+a.id);
 await expect(page.getByRole('dialog')).toContainText('First wide table');
 expect(await isModal(page)).toBe(false);
 await expect(page.getByRole('group',{name:m('room.view'),exact:true})).toBeHidden();
 await page.getByRole('link',{name:/Second wide table/}).click();
 await expect(page).toHaveURL(new RegExp('/tasks/'+b.id));
 await expect(page.getByRole('dialog')).toContainText('Second wide table');
});

test('at 900 px the Floor | Activity switch replaces the side column',async({page,service,activate})=>{
 await page.setViewportSize({width:900,height:800});
 await activate();
 const p=await seedProject(page,service);await seedTask(page,service,p,'integrator','Medium table');
 await page.goto(service.origin+'/rooms/'+p.id);
 const view=page.getByRole('group',{name:m('room.view'),exact:true});
 await expect(view).toBeVisible();
 await expect(page.getByRole('link',{name:/Medium table/})).toBeVisible();
 await view.getByRole('button',{name:m('activity.title'),exact:true}).click();
 await expect(view.getByRole('button',{name:m('activity.title'),exact:true})).toHaveAttribute('aria-pressed','true');
 await expect(page.getByRole('link',{name:/Medium table/})).toBeHidden();
});

test('at 375 px tabs scroll in their own strip, truncated names keep their full value, and Needs you and the drawer are full-screen',async({page,service,activate})=>{
 await page.setViewportSize({width:375,height:812});
 await activate();
 const long='A very long room name that cannot fit on a narrow phone screen';
 const p=await ownerCommand(page,service,'project.create',{name:long,repositories:[{name:'fixture-repo',reference:'synthetic://fixture'}]});
 for(const name of ['Second room','Third room','Fourth room','Fifth room'])await ownerCommand(page,service,'project.create',{name,repositories:[{name:'fixture-repo',reference:'synthetic://'+name.replaceAll(' ','-')}]});
 const task=await seedTask(page,service,p,'integrator','Narrow table');
 const agent=await enroll(page,service,p.id,'An agent with a remarkably long display name');await claimIntegrator(page,service,agent,task.id);
 await page.goto(service.origin+'/rooms/'+p.id);
 const tabs=page.getByRole('navigation',{name:m('shell.rooms')});
 await expect(tabs).toBeVisible();
 expect(await tabs.evaluate(n=>{const strip=n.querySelector('.room-tabs')??n;return strip.scrollWidth>strip.clientWidth})).toBe(true);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 await expect(page.locator('.room-tab .room-name',{hasText:long})).toHaveAttribute('title',long);
 await expect(page.locator('.seat-name').first()).toHaveAttribute('title','An agent with a remarkably long display name');
 await page.getByRole('button',{name:m('shell.needsYou')}).click();
 const panel=page.locator('#needs-you-panel');
 await expect(panel).toBeVisible();
 await panel.evaluate(el=>Promise.all(el.getAnimations().map(a=>a.finished)));
 expect(await panel.evaluate(el=>{const r=el.getBoundingClientRect();return [Math.round(r.width),Math.round(r.height)]})).toEqual([375,812]);
 await page.keyboard.press('Escape');
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
 await expect(page.getByRole('dialog')).toContainText('Narrow table');
 expect(await isModal(page)).toBe(true);
 expect(await page.locator('dialog.drawer').evaluate(el=>{const r=el.getBoundingClientRect();return [Math.round(r.width),Math.round(r.height)]})).toEqual([375,812]);
});

test('the drawer animates in, and has no transition with reduced motion',async({page,service,activate})=>{
 await page.setViewportSize({width:1280,height:800});
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Motion table');
 const durations=()=>page.locator('dialog.drawer').evaluate(d=>getComputedStyle(d).transitionDuration.split(',').map(s=>s.trim()));
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
 await expect(page.getByRole('dialog')).toContainText('Motion table');
 const normal=await durations();
 expect(normal.some(d=>d!=='0s')).toBe(true);
 expect(normal.every(d=>parseFloat(d)<=0.2)).toBe(true);
 await page.emulateMedia({reducedMotion:'reduce'});
 expect((await durations()).every(d=>d==='0s')).toBe(true);
});
