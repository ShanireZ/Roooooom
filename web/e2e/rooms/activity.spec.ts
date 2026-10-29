import {test,expect} from '../fixture';
import {agentCommand,enroll,ownerRead,seedProject} from '../helpers';
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
