import {test,expect} from '../fixture';
import {agentCommand,claimIntegrator,enroll,ownerRead,seedProject,seedTask} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

const feed=(page:import('@playwright/test').Page)=>page.getByRole('list',{name:m('activity.feed')});

test('ask mode creates a door note',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:m('room.ask')}).click();
 await expect(page.getByLabel(m('composer.askLabel'))).toBeFocused();
 await page.getByLabel(m('composer.askLabel')).fill('Weekly summary');
 await page.getByRole('button',{name:m('composer.askSubmit')}).click();
 await expect(page.getByRole('list',{name:m('room.door')})).toContainText('Weekly summary');
 await expect(feed(page)).toContainText('Weekly summary');
});

test('say mode posts a chat entry; an agent relaying approval stays chat',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const agent=await enroll(page,service,p.id,'Codex');
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:m('composer.say')}).click();
 await page.getByLabel(m('composer.sayLabel')).fill('Please read the request first.');
 await page.getByRole('button',{name:m('composer.saySubmit')}).click();
 const mine=feed(page).getByRole('listitem').filter({hasText:'Please read the request first.'});
 await expect(mine).toContainText(m('activity.you'));
 await expect(mine).toContainText(m('activity.chat'));
 const thread=await ownerRead(page,service,'thread',undefined,p.id);
 expect(thread[0]).toMatchObject({purpose:'discussion',priority:'normal',actionRequired:false});
 await agentCommand(agent,'message.send',{projectId:p.id,threadId:'project',purpose:'discussion',priority:'normal',body:'The owner approved this in chat.',recipientMode:'participants',recipientIds:[agent.participantId],actionRequired:false});
 const relay=feed(page).getByRole('listitem').filter({hasText:'The owner approved this in chat.'});
 await expect(relay).toContainText(m('activity.chat'));
 await expect(relay).toContainText('Codex');
 await expect(relay.locator('.stamp-formal')).toHaveCount(0);
});

test('filters narrow the feed',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:m('room.ask')}).click();
 await page.getByLabel(m('composer.askLabel')).fill('Request entry');
 await page.getByRole('button',{name:m('composer.askSubmit')}).click();
 await page.getByRole('button',{name:m('composer.say')}).click();
 await page.getByLabel(m('composer.sayLabel')).fill('Chat entry');
 await page.getByRole('button',{name:m('composer.saySubmit')}).click();
 await expect(feed(page)).toContainText('Chat entry');
 await page.getByRole('button',{name:m('activity.filter.chat')}).click();
 await expect(feed(page)).toContainText('Chat entry');
 await expect(feed(page)).not.toContainText('Request entry');
 await page.getByRole('button',{name:m('activity.filter.progress')}).click();
 await expect(feed(page)).toContainText('Request entry');
 await expect(feed(page)).not.toContainText('Chat entry');
 await page.getByRole('button',{name:m('activity.filter.all')}).click();
 await expect(feed(page)).toContainText('Chat entry');
});

test('service loss during ask submit keeps every requirement and retries exactly once',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const a=await enroll(page,service,p.id,'Designated integrator');
 await page.goto(service.origin+'/rooms/'+p.id);
 let lost=true;const requests:unknown[]=[];
 await page.route('**/commands/request.create',async route=>{requests.push(route.request().postDataJSON());if(lost){lost=false;await route.fetch();await route.abort('failed')}else await route.continue()});
 await page.getByRole('button',{name:m('room.ask')}).click();
 await page.getByLabel(m('composer.askLabel')).fill('Keep the original request');
 await page.getByText(m('composer.more')).click();
 await page.getByLabel(m('composer.constraints')).fill('Keep the login method');
 await page.getByLabel(m('composer.limit')).fill('2');
 await page.getByLabel(m('composer.integrator')).selectOption(a.participantId);
 await page.getByLabel(m('composer.conditions')).fill('Exact-version tests pass');
 await page.getByLabel(m('composer.acceptor')).selectOption('human');
 await page.getByRole('button',{name:m('composer.askSubmit')}).click();
 await expect(page.getByRole('alert')).toBeVisible();
 const retry=page.getByRole('button',{name:m('composer.retry')});
 await expect(retry).toBeEnabled({timeout:10000});
 expect(requests).toHaveLength(1);
 await retry.click();
 await expect(page.getByRole('list',{name:m('room.door')})).toContainText('Keep the original request');
 expect(requests).toHaveLength(2);
 expect(requests[0]).toEqual(requests[1]);
 const saved=await ownerRead(page,service,'requests',undefined,p.id);
 expect(saved).toHaveLength(1);
 expect(saved[0]).toMatchObject({constraints:['Keep the login method'],collaboratorLimit:2,designatedIntegratorId:a.participantId,acceptancePolicy:{mode:'human',conditions:['Exact-version tests pass']}});
});

test('a task-thread message shows its body in the room feed with a link to its task',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Thread table');
 const agent=await enroll(page,service,p.id,'Codex');await claimIntegrator(page,service,agent,task.id);
 await agentCommand(agent,'message.send',{projectId:p.id,taskId:task.id,threadId:'task',purpose:'discussion',priority:'normal',body:'Task thread note for the owner.',recipientMode:'participants',recipientIds:[agent.participantId],actionRequired:false});
 await page.goto(service.origin+'/rooms/'+p.id);
 const entry=feed(page).getByRole('listitem').filter({hasText:'Task thread note for the owner.'});
 await expect(entry).toContainText('Codex');
 await expect(entry.getByRole('link',{name:'Thread table'})).toHaveAttribute('href','/rooms/'+p.id+'/tasks/'+task.id+'/discussion');
});

test('project chat in the feed shows its purpose, a handling request and delivery status',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const a=await enroll(page,service,p.id,'Codex'),b=await enroll(page,service,p.id,'Gemini');
 await agentCommand(a,'message.send',{projectId:p.id,threadId:'project',purpose:'delivery',priority:'normal',body:'Please review the delivered pages.',recipientMode:'participants',recipientIds:[b.participantId],actionRequired:true});
 await page.goto(service.origin+'/rooms/'+p.id);
 const entry=feed(page).getByRole('listitem').filter({hasText:'Please review the delivered pages.'});
 await expect(entry).toContainText(m('purpose.delivery'));
 await expect(entry).toContainText(m('discussion.actionRequired'));
 await entry.getByText(m('discussion.receipts')).click();
 await expect(entry).toContainText('Gemini');
 await expect(entry).toContainText('stored');
});

test('loading earlier activity keeps the reader in place instead of jumping to the newest entry',async({page,service,activate})=>{
 await page.setViewportSize({width:1440,height:900});
 await activate();
 const p=await seedProject(page,service),agent=await enroll(page,service,p.id,'Codex');
 for(let i=0;i<105;i++)await agentCommand(agent,'message.send',{projectId:p.id,threadId:'project',purpose:'status',priority:'normal',body:'Status note '+i,recipientMode:'participants',recipientIds:[agent.participantId],actionRequired:false});
 await page.goto(service.origin+'/rooms/'+p.id);
 const list=feed(page);
 await expect(list).toContainText('Status note 104');
 await list.evaluate(el=>{el.scrollTop=0});
 await list.getByRole('button',{name:m('activity.older')}).click();
 await expect(list).toContainText('Status note 0');
 await page.waitForTimeout(300);
 const fromBottom=await list.evaluate(el=>el.scrollHeight-el.scrollTop-el.clientHeight);
 expect(fromBottom).toBeGreaterThan(1000);
});

test('a kept request whose result is unknown can be discarded',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 await page.goto(service.origin+'/rooms/'+p.id);
 let lost=true;
 await page.route('**/commands/request.create',async route=>{if(lost){lost=false;await route.abort('failed')}else await route.continue()});
 await page.getByRole('button',{name:m('room.ask')}).click();
 const text=page.getByLabel(m('composer.askLabel'));
 await text.fill('A request I no longer want');
 await page.getByRole('button',{name:m('composer.askSubmit')}).click();
 const discard=page.getByRole('button',{name:m('composer.discard')});
 await expect(discard).toBeEnabled({timeout:10000});
 await discard.click();
 await expect(page.getByText(m('composer.discarded'))).toBeVisible();
 await expect(page.getByRole('button',{name:m('composer.retry')})).toHaveCount(0);
 await expect(text).toHaveValue('');
 await expect(text).toBeEditable();
 await text.fill('The request I want');
 await page.getByRole('button',{name:m('composer.askSubmit')}).click();
 await expect(page.getByRole('list',{name:m('room.door')})).toContainText('The request I want');
 expect((await ownerRead(page,service,'requests',undefined,p.id)).map((r:{text:string})=>r.text)).toEqual(['The request I want']);
});
