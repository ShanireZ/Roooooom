import {test,expect} from '../fixture';
import {agentCommand,claimIntegrator,enroll,seedProject,seedTask} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

// Per-browser conveniences the UI may keep; nothing else, and never a secret.
const STORAGE=['roooooom.locale','roooooom.theme','roooooom.lastRoom'];
const storage=(page:import('@playwright/test').Page)=>page.evaluate(()=>({keys:Object.keys(localStorage).concat(Object.keys(sessionStorage)),values:JSON.stringify({...localStorage,...sessionStorage})}));

test('closing the browser page keeps the own local service available',async({page,service,activate})=>{
 await activate();await page.close();
 const response=await fetch(service.origin+'/identity');expect(response.ok).toBe(true);expect((await response.json()).status).toBe('ok');
});

test('owner activation, refresh and deep-link use the real embedded application',async({page,service,activate})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(service.origin+'/');
 await expect(page.getByRole('heading',{name:m('notActivated.title'),exact:true})).toBeVisible();
 await activate();
 await expect(page).toHaveURL(service.origin+'/rooms/new');
 const cookies=await page.context().cookies();
 expect(cookies.find(c=>c.name==='roooooom_owner')).toMatchObject({httpOnly:true,sameSite:'Strict'});
 const me=(await (await page.request.get(service.origin+'/api/v1/me')).json()).result;
 const kept=await storage(page);
 expect(kept.keys.filter(k=>!STORAGE.includes(k))).toEqual([]);
 expect(kept.values).not.toContain(me.csrf);
 await page.reload();await expect(page.getByRole('heading',{name:m('setup.title'),exact:true})).toBeVisible();
 const project=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+project.id);await page.reload();
 await expect(page.getByRole('heading',{name:'Protocol rehearsal'})).toBeVisible();
 await page.setViewportSize({width:390,height:844});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 await page.goto(service.origin+'/rooms/'+project.id+'/tasks/synthetic-task');await page.reload();
 await expect(page.getByRole('dialog')).toContainText(m('task.notFound'));
 expect(errors).toEqual([]);
});

test('foreign Origin, project files and guessed secrets are rejected',async({page,service,activate})=>{
 await activate();
 const me=await page.request.get(service.origin+'/api/v1/me');const payload=await me.json();
 const identity=await (await page.request.get(service.origin+'/identity')).json();
 const r=await page.request.post(service.origin+'/api/v1/commands/project.create',{
  headers:{Origin:'https://foreign.invalid','X-CSRF-Token':payload.result.csrf},
  data:{serviceId:identity.serviceId,datasetGeneration:identity.datasetGeneration,clientRequestId:'foreign',payload:{name:'forbidden',repositories:[]}}
 });
 expect(r.status()).toBe(403);
 for(const path of ['/AGENTS.md','/.git/config','/credentials/owner.json','/assets/../../PLAN.md','/assets/missing.js']){
  const response=await page.request.get(service.origin+path);expect(response.status()).toBe(404);
 }
});

test('a held older query cannot suppress a fresh read after an event',async({page,service,activate})=>{
 await activate();
 let release!:()=>void,seen!:()=>void,first=true;
 const gate=new Promise<void>(r=>{release=r});
 const captured=new Promise<void>(r=>{seen=r});
 await page.route('**/api/v1/queries/projects',async route=>{
  const response=await route.fetch();
  if(first){first=false;seen();await gate}
  await route.fulfill({response});
 });
 try{
  await page.goto(service.origin+'/rooms/new');await captured;
  const identity=await (await page.request.get(service.origin+'/identity')).json();
  const me=await (await page.request.get(service.origin+'/api/v1/me')).json();
  const created=await page.request.post(service.origin+'/api/v1/commands/project.create',{
   headers:{Origin:service.origin,'X-CSRF-Token':me.result.csrf},
   data:{serviceId:identity.serviceId,datasetGeneration:identity.datasetGeneration,clientRequestId:crypto.randomUUID(),payload:{name:'Newer committed project',repositories:[]}}
  });
  expect(created.status()).toBe(200);
  await expect(page.getByRole('navigation',{name:m('shell.rooms')}).getByRole('link',{name:'Newer committed project'})).toBeVisible();
 }finally{release()}
});

test('UnsafeMarkdownAndDisconnectedUI: messages render as text with their actual actor',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service),agent=await enroll(page,service,project.id,'Relay agent');
 const task=await seedTask(page,service,project);await claimIntegrator(page,service,agent,task.id);
 const body='<script data-injected>window.injected=true</script><img src=x onerror="window.injected=true">';
 await agentCommand(agent,'message.send',{projectId:project.id,taskId:task.id,threadId:'task',purpose:'discussion',priority:'normal',body,recipientMode:'participants',recipientIds:[agent.participantId],actionRequired:false});
 await page.goto(service.origin+'/rooms/'+project.id+'/tasks/'+task.id+'/discussion');
 const entry=page.getByRole('dialog').getByRole('listitem').filter({hasText:body});
 await expect(entry.getByText(body,{exact:true})).toBeVisible();
 await expect(page.locator('script[data-injected]')).toHaveCount(0);
 expect(await page.evaluate(()=>Reflect.get(window,'injected'))).toBeUndefined();
 await expect(entry).toContainText('Relay agent');
 await expect(entry).not.toContainText(m('activity.you'));
});

test('UnsafeMarkdownAndDisconnectedUI: outage keeps the snapshot read-only',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);await seedTask(page,service,project,'integrator','Table kept while offline');
 await page.goto(service.origin+'/rooms/'+project.id);
 await expect(page.getByRole('link',{name:/Table kept while offline/})).toBeVisible();
 await service.stop();
 await expect(page.getByRole('status').filter({hasText:m('shell.readOnly')})).toBeVisible({timeout:15000});
 await expect(page.getByRole('link',{name:/Table kept while offline/})).toBeVisible();
 await page.getByRole('button',{name:m('room.ask')}).click();
 await expect(page.getByRole('button',{name:m('composer.askSubmit')})).toBeDisabled();
 expect((await storage(page)).keys.filter(k=>!STORAGE.includes(k))).toEqual([]);
});
