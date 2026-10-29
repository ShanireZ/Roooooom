import {test,expect} from '../fixture';
import {askDecision,claimIntegrator,enroll,ownerCommand,seedProject,seedTask} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

const floorList=(page:import('@playwright/test').Page)=>page.getByRole('list',{name:m('room.floor')});

test('tables, notes, seats and lobby reflect records',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const cx=await enroll(page,service,p.id,'Codex');
 await enroll(page,service,p.id,'Claude Code');
 const task=await seedTask(page,service,p,'integrator','Article table of contents');
 await claimIntegrator(page,service,cx,task.id);
 await ownerCommand(page,service,'request.create',{projectId:p.id,text:'RSS feed'});
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(page.getByRole('list',{name:m('room.door')})).toContainText('RSS feed');
 const item=floorList(page).getByRole('listitem').filter({hasText:'Article table of contents'});
 await expect(item.getByRole('link',{name:/Article table of contents/})).toBeVisible();
 await expect(item).toContainText('Codex');
 await expect(item).toContainText(m('seat.role.integrating'));
 const lobby=page.getByRole('list',{name:m('room.lobby')});
 await expect(lobby).toContainText('Claude Code');
 await expect(lobby).not.toContainText('Codex');
 await expect(page.getByRole('link',{name:new RegExp(m('room.cabinet'))})).toBeVisible();
});

test('stale seat is dashed with last-synced text',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const cx=await enroll(page,service,p.id,'Codex');
 const task=await seedTask(page,service,p,'integrator','Search');
 await claimIntegrator(page,service,cx,task.id);
 await page.clock.install();
 await page.goto(service.origin+'/rooms/'+p.id);
 const seat=floorList(page).getByRole('button',{name:/Codex/});
 await expect(seat).toHaveAttribute('data-stale','false');
 await page.clock.fastForward('16:00');
 await expect(seat).toHaveAttribute('data-stale','true');
 await expect(seat).toContainText(/16 minutes ago/);
});

test('keyboard arrows move between tables and Enter opens the task drawer',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const first=await seedTask(page,service,p,'integrator','First table');
 await seedTask(page,service,p,'integrator','Second table');
 await page.goto(service.origin+'/rooms/'+p.id);
 const links=floorList(page).getByRole('link');
 await links.first().focus();
 await expect(links.first()).toContainText('First table');
 await page.keyboard.press('ArrowRight');
 await expect(links.nth(1)).toBeFocused();
 await page.keyboard.press('ArrowLeft');
 await expect(links.first()).toBeFocused();
 await page.keyboard.press('Enter');
 await expect(page).toHaveURL(service.origin+'/rooms/'+p.id+'/tasks/'+first.id);
});

test('member focus dims other tables',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const cx=await enroll(page,service,p.id,'Codex');
 await enroll(page,service,p.id,'Gemini CLI');
 const a=await seedTask(page,service,p,'integrator','Codex work');
 await claimIntegrator(page,service,cx,a.id);
 await seedTask(page,service,p,'integrator','Unclaimed work');
 await page.goto(service.origin+'/rooms/'+p.id);
 const codexTable=floorList(page).getByRole('listitem').filter({hasText:'Codex work'});
 const otherTable=floorList(page).getByRole('listitem').filter({hasText:'Unclaimed work'});
 await codexTable.getByRole('button',{name:/Codex/}).click();
 await expect(otherTable).toHaveAttribute('data-dimmed','true');
 await expect(codexTable).toHaveAttribute('data-dimmed','false');
 await page.getByRole('button',{name:m('seat.clearFocus')}).click();
 await expect(otherTable).toHaveAttribute('data-dimmed','false');
});

test('long mixed-script text never scrolls the page horizontally at 375px',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const cx=await enroll(page,service,p.id,'エージェント測試🤖'.repeat(4));
 const goal='Long goal '+'長文目錄與搜尋功能🙂'.repeat(180);
 const task=await seedTask(page,service,p,'integrator',goal);
 await claimIntegrator(page,service,cx,task.id);
 await page.setViewportSize({width:375,height:800});
 await page.goto(service.origin+'/rooms/'+p.id);
 const link=floorList(page).getByRole('link').first();
 await expect(link).toHaveAttribute('title',goal);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
});

test('a room reads assignments with one project request, never per table',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 for(const goal of ['One','Two','Three'])await seedTask(page,service,p,'integrator',goal);
 const urls:string[]=[];
 page.on('request',r=>{if(r.url().includes('/queries/assignments'))urls.push(r.url())});
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(floorList(page).getByRole('link')).toHaveCount(3);
 expect(urls.filter(u=>u.includes('targetId='))).toHaveLength(0);
 expect(urls.filter(u=>u.includes('projectId='+p.id)).length).toBeGreaterThanOrEqual(1);
});

test('a focused table keeps its position when a new owner flag arrives',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const cx=await enroll(page,service,p.id,'Codex');
 const ids:Record<string,string>={};
 for(const goal of ['Alpha','Bravo','Charlie']){const t=await seedTask(page,service,p,'integrator',goal);ids[goal]=t.id;await claimIntegrator(page,service,cx,t.id)}
 await page.goto(service.origin+'/rooms/'+p.id);
 const order=async()=>(await floorList(page).getByRole('link').allTextContents()).map(text=>['Alpha','Bravo','Charlie'].find(g=>text.includes(g)));
 await expect.poll(order).toEqual(['Charlie','Bravo','Alpha']);
 await floorList(page).getByRole('link',{name:/Bravo/}).focus();
 await askDecision(page,service,cx,ids.Alpha);
 await expect(floorList(page).getByRole('listitem').filter({hasText:'Alpha'})).toContainText(m('flag.decision'));
 expect(await order()).toEqual(['Charlie','Bravo','Alpha']);
 await page.evaluate(()=>(document.activeElement as HTMLElement|null)?.blur());
 await expect.poll(order).toEqual(['Alpha','Charlie','Bravo']);
});

test('a hovered floor stays held after keyboard focus leaves it',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const cx=await enroll(page,service,p.id,'Codex');
 const ids:Record<string,string>={};
 for(const goal of ['Alpha','Bravo','Charlie']){const t=await seedTask(page,service,p,'integrator',goal);ids[goal]=t.id;await claimIntegrator(page,service,cx,t.id)}
 await page.goto(service.origin+'/rooms/'+p.id);
 const order=async()=>(await floorList(page).getByRole('link').allTextContents()).map(text=>['Alpha','Bravo','Charlie'].find(g=>text.includes(g)));
 await expect.poll(order).toEqual(['Charlie','Bravo','Alpha']);
 const bravo=floorList(page).getByRole('link',{name:/Bravo/});
 await bravo.hover();await bravo.focus();
 await askDecision(page,service,cx,ids.Alpha);
 await expect(floorList(page).getByRole('listitem').filter({hasText:'Alpha'})).toContainText(m('flag.decision'));
 await page.evaluate(()=>(document.activeElement as HTMLElement|null)?.blur());
 await page.waitForTimeout(400);
 expect(await order()).toEqual(['Charlie','Bravo','Alpha']);
 await page.mouse.move(5,5);
 await expect.poll(order).toEqual(['Alpha','Charlie','Bravo']);
});
