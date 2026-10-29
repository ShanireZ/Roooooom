import {test,expect} from '../fixture';
import {agentCommand,claimIntegrator,enroll,ownerCommand,ownerRead,seedProject,seedTask,taskVersions} from '../helpers';
import {m} from '../i18n';

// Old owner-UI invariants carried into the room model under their original names.
test.use({locale:'en-US'});

const drawer=(page:import('@playwright/test').Page)=>page.getByRole('dialog');

test('one sentence is durable while no agent is present, with two daily navigation entries',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+project.id);
 await expect(page.getByRole('navigation',{name:m('shell.rooms')})).toBeVisible();
 await expect(page.getByRole('button',{name:new RegExp(m('shell.needsYou'))})).toBeVisible();
 await page.getByRole('button',{name:m('room.ask')}).click();
 const form=page.locator('form').filter({has:page.getByLabel(m('composer.askLabel'))});
 await expect(form.locator('input:required,textarea:required,select:required')).toHaveCount(1);
 await page.getByLabel(m('composer.askLabel')).fill('Let the weekly report support Chinese titles and keep the export format.');
 await page.getByRole('button',{name:m('composer.askSubmit')}).click();
 const door=page.getByRole('list',{name:m('room.door')});
 await expect(door).toContainText(m('request.waiting'));
 const requests=await ownerRead(page,service,'requests',undefined,project.id);
 expect(requests).toHaveLength(1);expect(requests[0].text).toContain('Chinese titles');
 expect(await ownerRead(page,service,'tasks')).toHaveLength(0);
 await page.reload();await expect(door).toContainText('Chinese titles');
 await page.setViewportSize({width:390,height:844});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('ordinary chat needs only text and same-name members remain distinct',async({page,service,activate})=>{
 await activate();const p=await seedProject(page,service);
 const a=await enroll(page,service,p.id,'Same-name agent'),b=await enroll(page,service,p.id,'Same-name agent');
 await page.goto(service.origin+'/rooms/'+p.id+'/settings/members');
 const members=page.getByRole('list',{name:m('settings.members'),exact:true});
 await expect(members).toContainText(a.participantId);await expect(members).toContainText(b.participantId);
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:m('composer.say')}).click();
 const form=page.locator('form').filter({has:page.getByLabel(m('composer.sayLabel'))});
 await expect(form.locator('input:required,textarea:required,select:required')).toHaveCount(1);
 await page.getByLabel(m('composer.sayLabel')).fill('Read the request first; product trade-offs are decided separately.');
 await page.getByRole('button',{name:m('composer.saySubmit')}).click();
 await expect(page.getByRole('list',{name:m('activity.feed')})).toContainText('Read the request first');
 const messages=await ownerRead(page,service,'thread',undefined,p.id);
 expect(messages).toHaveLength(1);expect(messages[0]).toMatchObject({purpose:'discussion',priority:'normal',actionRequired:false});
});

test('a retry after a committed response is lost does not create a second task',async({page,service,activate})=>{
 await activate();const p=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:m('composer.fullSpec')}).click();
 const editor=page.getByRole('dialog',{name:m('create.title')});
 await editor.getByLabel(m('create.goal'),{exact:true}).fill('One committed task');
 await editor.getByLabel(m('create.scope'),{exact:true}).fill('fixture');
 await editor.getByLabel(m('create.nonScope'),{exact:true}).fill('publication');
 await editor.getByLabel(m('create.authority'),{exact:true}).fill('synthetic://owner');
 await editor.getByLabel(m('create.conditions'),{exact:true}).fill('test');
 let drop=true;const ids:string[]=[];
 await page.route('**/api/v1/commands/task.create',async route=>{ids.push(route.request().postDataJSON().clientRequestId);const response=await route.fetch();if(drop){drop=false;await route.abort('failed')}else await route.fulfill({response})});
 await editor.getByRole('button',{name:m('create.submit'),exact:true}).click();
 await expect(editor.getByRole('alert')).toContainText(m('error.offline'));
 await editor.getByRole('button',{name:m('create.submit'),exact:true}).click();
 await expect(page).toHaveURL(/\/tasks\//);
 expect(ids).toHaveLength(2);expect(ids[0]).toBe(ids[1]);
 expect((await ownerRead(page,service,'tasks')).filter((t:any)=>t.goal==='One committed task')).toHaveLength(1);
});

test('owner forced handoff requires explicit dispositions and keeps hosts outside the service',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);
 const old=await enroll(page,service,project.id,'Old integrator'),next=await enroll(page,service,project.id,'New integrator');
 const task=await seedTask(page,service,project,'integrator','Handoff with dispositions');await claimIntegrator(page,service,old,task.id);
 for(const role of ['transfer-this','release-this']){
  let current=await ownerRead(page,service,'task',task.id);
  const assignment=await ownerCommand(page,service,'assignment.create',{taskId:task.id,role,input:'fixture',output:'evidence',writeScope:[role+'/'],repoId:project.repositories[0].id,baseRef:'synthetic-base',dependencies:[]},undefined,taskVersions(current));
  current=await ownerRead(page,service,'task',task.id);
  await agentCommand(old,'assignment.claim',{},assignment.id,{...taskVersions(current),expectedVersion:assignment.version});
 }
 await page.goto(service.origin+'/rooms/'+project.id+'/tasks/'+task.id+'/manage');
 const d=drawer(page);
 await d.getByText(m('manage.handoff'),{exact:true}).click();
 await d.getByLabel(m('manage.handoffTarget')).selectOption(next.sessionId);
 await d.getByLabel(m('manage.disposition',{role:'transfer-this'})).selectOption('transfer');
 await d.getByLabel(m('manage.disposition',{role:'release-this'})).selectOption('release');
 await d.getByLabel(m('manage.handoffReason')).fill('Owner explicit recovery');
 await d.getByLabel(m('manage.handoffConfirm')).check();
 await d.getByRole('button',{name:m('manage.confirmHandoff')}).click();
 await expect.poll(async()=>(await ownerRead(page,service,'task',task.id)).integratorId).toBe(next.participantId);
 const assignments=await ownerRead(page,service,'assignments',task.id);
 expect(assignments.find((a:any)=>a.role==='transfer-this').claimSessionId).toBe(next.sessionId);
 expect(assignments.find((a:any)=>a.role==='release-this').claimSessionId).toBeUndefined();
});

test('owner stores masked integration credentials and explicitly enables a notification binding',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+project.id+'/settings/integrations');
 const d=drawer(page);
 await d.getByText(m('advanced.title'),{exact:true}).click();
 await d.getByLabel(m('advanced.credentialKind'),{exact:true}).selectOption('dingtalk');
 await d.getByLabel(m('advanced.token'),{exact:true}).fill('synthetic-webhook-token');
 await d.getByLabel(m('guided.secret'),{exact:true}).fill('SECsynthetic-test');
 const saved=page.waitForResponse(r=>r.url().endsWith('/commands/binding.credential'));
 await d.getByRole('button',{name:m('advanced.saveCredential'),exact:true}).click();
 const response=await (await saved).json();
 expect(JSON.stringify(response)).not.toContain('synthetic-webhook-token');expect(JSON.stringify(response)).not.toContain('SECsynthetic-test');
 await expect(d.getByLabel(m('advanced.token'),{exact:true})).toHaveValue('');
 await expect(d.getByLabel(m('advanced.dingtalkRef'),{exact:true})).toHaveValue(response.result.credentialRef);
 await expect(d.getByText(m('advanced.dingtalkLimits'),{exact:true})).toBeVisible();
 await d.getByRole('button',{name:m('advanced.createDingtalk'),exact:true}).click();
 await expect(d.getByText(m('advanced.dingtalkCreated'),{exact:true})).toBeVisible();
 const bindings=await ownerRead(page,service,'bindings',undefined,project.id);
 expect(bindings).toHaveLength(1);expect(bindings[0].kind).toBe('dingtalk');
 expect(JSON.stringify(bindings)).not.toContain('synthetic-webhook-token');
 // Enabling is not a test send: no outbound job exists.
 expect(await ownerRead(page,service,'outbound-jobs',undefined,project.id)).toHaveLength(0);
 await page.setViewportSize({width:390,height:844});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
});
