import {test,expect} from '../fixture';
import {agentCommand,askDecision,claimIntegrator,enroll,ownerCommand,ownerRead,seedProject,seedTask} from '../helpers';
import {m} from '../i18n';

// Refined blueprint (docs/ui-design.md §7, §10, §13): ratios, one type scale and motion.
test.use({locale:'en-US',viewport:{width:1440,height:900}});

type Page=import('@playwright/test').Page;
const box=(page:Page,selector:string)=>page.locator(selector).first().evaluate(el=>{const r=el.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,height:r.height,width:r.width}});
const animating=(page:Page,selector:string)=>page.locator(selector).first().evaluate(el=>el.getAnimations({subtree:true}).length>0);

test('the floor fills the window height and the lobby sits under the cabinet',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);await seedTask(page,service,p,'integrator','Height table');
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(page.getByRole('link',{name:/Height table/})).toBeVisible();
 expect((await box(page,'.floor-plan')).bottom).toBeGreaterThan(900-40);
 await expect(page.locator('.side-col .cabinet')).toBeVisible();
 await expect(page.locator('.side-col .lobby')).toBeVisible();
});

test('seats sit inside their table and tables in one row share a height',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const a=await seedTask(page,service,p,'integrator','Seated table');await seedTask(page,service,p,'integrator','Empty table');
 const agent=await enroll(page,service,p.id,'Codex');await claimIntegrator(page,service,agent,a.id);
 await page.goto(service.origin+'/rooms/'+p.id);
 const seated=page.locator('.table-item').filter({hasText:'Seated table'}),empty=page.locator('.table-item').filter({hasText:'Empty table'});
 await expect(seated.locator('.seat')).toBeVisible();
 const card=await seated.evaluate(el=>el.getBoundingClientRect().toJSON()),seat=await seated.locator('.seat').evaluate(el=>el.getBoundingClientRect().toJSON());
 expect(seat.bottom).toBeLessThanOrEqual(card.bottom);expect(seat.left).toBeGreaterThanOrEqual(card.left);
 const other=await empty.evaluate(el=>el.getBoundingClientRect().toJSON());
 expect(Math.abs(other.top-card.top)).toBeLessThan(2);expect(Math.abs(other.height-card.height)).toBeLessThan(2);
 expect(card.width/card.height).toBeGreaterThan(1.2);expect(card.width/card.height).toBeLessThan(2.4);
});

test('opening a drawer at 1440 px reflows the floor instead of covering it',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const tasks=[];for(const goal of ['First reflow table','Second reflow table','Third reflow table'])tasks.push(await seedTask(page,service,p,'integrator',goal));
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+tasks[0].id);
 await expect(page.getByRole('dialog')).toContainText('First reflow table');
 await page.waitForTimeout(450);
 const drawer=await box(page,'dialog.drawer');
 const rights=await page.locator('.tables .table-item').evaluateAll(els=>els.map(el=>el.getBoundingClientRect().right));
 for(const right of rights)expect(right).toBeLessThanOrEqual(drawer.left+1);
 expect((await box(page,'.side-col')).width).toBeLessThan(2);
});

test('room and drawer text use only the seven type sizes',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Type table');
 const agent=await enroll(page,service,p.id,'Codex');await claimIntegrator(page,service,agent,task.id);
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
 await expect(page.getByRole('dialog')).toContainText('Type table');
 const sizes=await page.evaluate(()=>[...new Set([...document.querySelectorAll('.rm *')].filter(el=>[...el.childNodes].some(n=>n.nodeType===3&&n.textContent!.trim())&&el.getClientRects().length&&!el.closest('.sr-only')).map(el=>parseFloat(getComputedStyle(el).fontSize)))].sort((a,b)=>a-b));
 for(const size of sizes)expect([11,12,13,15,17,20,26],String(sizes)).toContain(size);
});

test('the room tab indicator follows the current room',async({page,service,activate})=>{
 await activate();
 const first=await seedProject(page,service);
 const second=await ownerCommand(page,service,'project.create',{name:'Second room',repositories:[{name:'r',reference:'synthetic://second'}]});
 await page.goto(service.origin+'/rooms/'+first.id);
 const ink=page.locator('.room-tabs .tab-ink');
 await expect.poll(async()=>Math.round((await box(page,'.room-tabs .tab-ink')).left)).toBe(Math.round((await box(page,'.room-tab[aria-current=page]')).left));
 await page.getByRole('link',{name:'Second room'}).click();
 await expect(page).toHaveURL(new RegExp(second.id));
 await expect.poll(async()=>Math.round((await box(page,'.room-tabs .tab-ink')).left)).toBe(Math.round((await box(page,'.room-tab[aria-current=page]')).left));
 await expect(ink).toHaveCSS('transition-duration',/0\.28s/);
});

test('the drawer leaves with an exit animation before it closes',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Exit table');
 await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
 await expect(page.getByRole('dialog')).toContainText('Exit table');
 await page.evaluate(()=>{const w=window as unknown as {closing:boolean};w.closing=false;new MutationObserver(r=>{if(r.some(x=>(x.target as Element).getAttribute?.('data-closing')==='true'))w.closing=true}).observe(document.body,{subtree:true,attributes:true,attributeFilter:['data-closing']})});
 await page.getByRole('button',{name:m('drawer.close')}).click();
 await expect(page.getByRole('dialog')).toHaveCount(0);
 await expect(page).toHaveURL(service.origin+'/rooms/'+p.id);
 expect(await page.evaluate(()=>(window as unknown as {closing:boolean}).closing)).toBe(true);
});

test('live changes animate: counts pop, new activity is inked, new flags land and tables move',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const older=await seedTask(page,service,p,'integrator','Older table'),newer=await seedTask(page,service,p,'integrator','Newer table');
 const agent=await enroll(page,service,p.id,'Codex');await claimIntegrator(page,service,agent,older.id);
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(page.locator('.table-item').first()).toBeVisible();
 await expect(page.locator('.stamp.arrive')).toHaveCount(0);
 await expect(page.locator('.entry.fresh')).toHaveCount(0);
 await askDecision(page,service,agent,older.id);
 const flagged=page.locator('.table-item').filter({hasText:'Older table'});
 await expect(flagged.locator('.stamp.arrive')).toBeVisible();
 await expect(page.locator('.room-tab .count.pop')).toBeVisible();
 await expect(page.locator('.entry.fresh').first()).toBeVisible();
 await expect.poll(()=>page.locator('.tables .table-item').first().textContent()).toContain('Older table');
 expect(newer.id).toBeTruthy();
});

test('a converted door note morphs into its new table',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),agent=await enroll(page,service,p.id,'Codex');
 let request=await ownerCommand(page,service,'request.create',{projectId:p.id,text:'Morph this request'});
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(page.getByRole('list',{name:m('room.door')})).toContainText('Morph this request');
 await agentCommand(agent,'request.claim',{},request.id,{expectedVersion:request.version,claimEpoch:0});
 request=await ownerRead(page,service,'request',request.id);
 await page.evaluate(()=>{const w=window as unknown as {morphed:boolean};w.morphed=false;new MutationObserver(()=>{const item=[...document.querySelectorAll('.tables .table-item')].find(el=>el.textContent?.includes('Morph this request'));if(item&&item.getAnimations().some(x=>x.id==='morph'))w.morphed=true}).observe(document.body,{subtree:true,childList:true})});
 await agentCommand(agent,'request.convert',{task:{projectId:p.id,goal:'Morph this request',scope:['fixture'],nonScope:['publication'],authorityRefs:['synthetic://owner'],risk:'normal',acceptancePolicy:{mode:'integrator',conditions:['tests pass']},collaboratorLimit:0}},request.id,{expectedVersion:request.version,claimEpoch:request.preparationEpoch});
 await expect(page.locator('.tables')).toContainText('Morph this request');
 await expect(page.getByRole('list',{name:m('room.door')})).not.toContainText('Morph this request');
 await expect.poll(()=>page.evaluate(()=>(window as unknown as {morphed:boolean}).morphed)).toBe(true);
});

test('with reduced motion nothing animates and the state still updates',async({page,service,activate})=>{
 await page.emulateMedia({reducedMotion:'reduce'});
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Still table');
 const agent=await enroll(page,service,p.id,'Codex');await claimIntegrator(page,service,agent,task.id);
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(page.locator('.table-item').first()).toBeVisible();
 await askDecision(page,service,agent,task.id);
 await expect(page.locator('.table-item').filter({hasText:'Still table'}).locator('.stamp')).toBeVisible();
 expect(await animating(page,'.rm')).toBe(false);
 await expect(page.locator('.room-tabs .tab-ink')).toHaveCSS('transition-duration','0s');
});
