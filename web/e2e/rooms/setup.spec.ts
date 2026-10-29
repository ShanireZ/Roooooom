import {readFile} from 'node:fs/promises';
import {test,expect} from '../fixture';
import {enroll,ownerRead,seedProject} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

const drawer=(page:import('@playwright/test').Page)=>page.getByRole('dialog');
const settings=(service:{origin:string},projectId:string,section:string)=>service.origin+'/rooms/'+projectId+'/settings/'+section;

test('one screen creates a room, opens the door and hands over join instructions',async({page,service,activate})=>{
 await activate();await page.goto(service.origin+'/rooms/new');
 await page.getByLabel(m('setup.reference'),{exact:true}).fill('C:/Projects/SimpleProject');
 await expect(page.getByLabel(m('setup.name'),{exact:true})).toHaveValue('SimpleProject');
 await page.getByLabel(m('setup.name'),{exact:true}).fill('Simple room');
 await page.getByLabel(m('setup.reference'),{exact:true}).fill('C:/Projects/SimpleProject2');
 await expect(page.getByLabel(m('setup.name'),{exact:true})).toHaveValue('Simple room');
 await expect(page.getByText(m('door.consent',{name:'Simple room'}),{exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:m('guided.github')})).toHaveCount(0);
 await page.getByRole('button',{name:m('setup.submit'),exact:true}).click();
 const join=page.getByTestId('join-instructions');
 await expect(join).toContainText('Simple room');
 await expect(join).toContainText('SKILL.md');
 const projects=await ownerRead(page,service,'projects');
 expect(projects).toHaveLength(1);expect(projects[0].enrollmentEnabled).toBe(true);
 await expect(join).toContainText('--project '+projects[0].id);
 await expect(page.locator('input[type=password]')).toHaveCount(0);
 await expect(page.getByRole('button',{name:m('guided.github')})).toHaveCount(0);
 await expect(page.getByText(m('setup.waiting'))).toBeVisible();
 await enroll(page,service,projects[0].id,'First agent');
 await expect(page.getByText(m('setup.joined',{name:'First agent'}))).toBeVisible();
});

test('lost room-create response retries the original request without a duplicate room',async({page,service,activate})=>{
 await activate();let lose=true;const ids:string[]=[];
 await page.route('**/api/v1/commands/project.create',async route=>{ids.push(route.request().postDataJSON().clientRequestId);if(lose){lose=false;await route.fetch();await route.abort('failed')}else await route.continue()});
 await page.goto(service.origin+'/rooms/new');
 await page.getByLabel(m('setup.reference'),{exact:true}).fill('synthetic://NoDuplicates');
 await page.getByRole('button',{name:m('setup.submit'),exact:true}).click();
 await expect(page.getByText(m('setup.uncertain'),{exact:true})).toBeVisible();
 await expect(page.getByLabel(m('setup.reference'),{exact:true})).not.toBeEditable();
 await page.getByRole('button',{name:m('setup.retry'),exact:true}).click();
 await expect(page.getByTestId('join-instructions')).toContainText('NoDuplicates');
 expect(ids).toHaveLength(2);expect(ids[0]).toBe(ids[1]);
 expect(await ownerRead(page,service,'projects')).toHaveLength(1);
});

test('a failed door opening leaves the room registered and the room banner opens it later',async({page,service,activate})=>{
 await activate();let fail=true;
 await page.route('**/api/v1/commands/enrollment.enable',async route=>{if(fail){fail=false;await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'temporary-unavailable'}})})}else await route.continue()});
 await page.goto(service.origin+'/rooms/new');
 await page.getByLabel(m('setup.reference'),{exact:true}).fill('synthetic://ResumeProject');
 await page.getByRole('button',{name:m('setup.submit'),exact:true}).click();
 await expect(page.getByText(m('setup.registeredClosed'),{exact:true})).toBeVisible();
 const [project]=await ownerRead(page,service,'projects');
 expect(project.enrollmentEnabled).toBe(false);
 await page.goto(service.origin+'/rooms/'+project.id);
 const banner=page.locator('.door-banner');
 await expect(banner).toContainText(m('door.banner'));
 await banner.getByRole('button',{name:m('door.open'),exact:true}).click();
 await expect(banner).toHaveCount(0);
 const projects=await ownerRead(page,service,'projects');
 expect(projects).toHaveLength(1);expect(projects[0].enrollmentEnabled).toBe(true);
});

test('room settings door shows repository references, asks consent, then shows the same join instructions',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);
 await page.goto(settings(service,project.id,'door').replace(/\/door$/,''));
 const d=drawer(page);
 await expect(d.getByRole('link',{name:m('settings.door'),exact:true})).toHaveAttribute('aria-current','page');
 await expect(d).toContainText(project.repositories[0].reference);
 await expect(d.getByText(m('door.closed'),{exact:true})).toBeVisible();
 await expect(d.getByText(m('door.consent',{name:project.name}),{exact:true})).toBeVisible();
 await expect(d.getByTestId('join-instructions')).toHaveCount(0);
 await d.getByRole('button',{name:m('door.open'),exact:true}).click();
 await expect(d.getByTestId('join-instructions')).toContainText('--project '+project.id);
 expect((await ownerRead(page,service,'projects'))[0].enrollmentEnabled).toBe(true);
});

test('room members get a private recovery file and are revoked only after an inline confirmation',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);const original=await enroll(page,service,project.id,'Member agent');
 await page.goto(settings(service,project.id,'members'));
 const row=drawer(page).getByRole('listitem').filter({hasText:'Member agent'});
 const recoveryResponse=page.waitForResponse(r=>r.url().endsWith('/commands/participant.recover'));
 await row.getByRole('button',{name:m('members.recover'),exact:true}).click();
 const recovery=(await (await recoveryResponse).json()).result;
 const credentials=JSON.parse(await readFile(recovery.credentialPath,'utf8'));
 expect(credentials.participantId).toBe(original.participantId);expect(credentials.sessionId).not.toBe(original.sessionId);
 await expect(drawer(page).getByText(m('members.recovered'),{exact:true})).toBeVisible();
 await expect(drawer(page)).toContainText(m('members.privatePath'));
 await row.getByRole('button',{name:m('members.revoke'),exact:true}).click();
 await expect(row.getByText(m('members.revokeConfirm'),{exact:true})).toBeVisible();
 expect((await fetch(service.origin+'/api/v1/me',{headers:{Authorization:'Bearer '+credentials.token}})).status).toBe(200);
 await row.getByRole('button',{name:m('members.confirmRevoke'),exact:true}).click();
 await expect(drawer(page).getByText(m('members.revoked'),{exact:true})).toBeVisible();
 expect((await fetch(service.origin+'/api/v1/me',{headers:{Authorization:'Bearer '+credentials.token}})).status).toBe(401);
});

test('failed metadata inspection resumes without storing the credential twice',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);let credentials=0,inspections=0;
 page.on('request',r=>{if(r.url().endsWith('/binding.credential'))credentials++});
 await page.route('**/commands/binding.inspect',async route=>{inspections++;await route.fulfill({status:inspections===1?409:200,contentType:'application/json',body:JSON.stringify(inspections===1?{error:{code:'github-metadata-unavailable'}}:{result:{repository:'Owner/Repo',repositoryId:42,private:false,hasIssues:true,issuesWriteVerified:false}})})});
 await page.goto(settings(service,project.id,'integrations'));
 const d=drawer(page);
 await d.getByRole('button',{name:m('guided.github'),exact:true}).click();
 await d.getByLabel(m('guided.repoUrl'),{exact:true}).fill('Owner/Repo');
 await d.getByLabel(m('guided.pat'),{exact:true}).fill('synthetic-retry');
 await d.getByRole('button',{name:m('guided.confirmDestination'),exact:true}).click();
 await expect(d.getByRole('alert')).toBeVisible();
 await expect(d.getByText(m('guided.credentialKept'),{exact:true})).toBeVisible();
 await d.getByRole('button',{name:m('guided.confirmDestination'),exact:true}).click();
 await expect(d.getByText(m('guided.notVerified'),{exact:true})).toBeVisible();
 expect(credentials).toBe(1);expect(inspections).toBe(2);
 await d.getByRole('button',{name:m('guided.enableGithub'),exact:true}).click();
 await expect(d.getByText(m('guided.githubEnabled'),{exact:true})).toBeVisible();
});

test('guided GitHub confirms metadata without claiming Issues write permission',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);
 let calls=0;await page.route('**/commands/binding.inspect',async route=>{calls++;await route.fulfill({contentType:'application/json',body:JSON.stringify({result:{repository:'Owner/Repo',repositoryId:42,private:true,hasIssues:true,issuesWriteVerified:false}})})});
 await page.goto(settings(service,project.id,'integrations'));
 const d=drawer(page);
 await d.getByRole('button',{name:m('guided.github'),exact:true}).click();
 await d.getByLabel(m('guided.repoUrl'),{exact:true}).fill('https://github.com/Owner/Repo');
 await d.getByLabel(m('guided.pat'),{exact:true}).fill('synthetic-pat');
 await d.getByRole('button',{name:m('guided.confirmDestination'),exact:true}).click();
 await expect(d.getByText(m('guided.notVerified'),{exact:true})).toBeVisible();expect(calls).toBe(1);
 await expect(d.getByText(m('guided.destination',{repository:'Owner/Repo',visibility:m('guided.private')}),{exact:true})).toBeVisible();
 expect(await ownerRead(page,service,'bindings',undefined,project.id)).toHaveLength(0);
 await d.getByRole('button',{name:m('guided.enableGithub'),exact:true}).click();
 await expect(d.getByText(m('guided.githubEnabled'),{exact:true})).toBeVisible();
 const [binding]=await ownerRead(page,service,'bindings',undefined,project.id);
 expect(binding.repositoryId).toBe(42);expect(binding.repository).toBe('Owner/Repo');
 expect(await page.evaluate(()=>JSON.stringify({...localStorage,...sessionStorage}))).not.toContain('synthetic-pat');
});

test('guided DingTalk accepts complete official URL without a test send',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);
 await page.goto(settings(service,project.id,'integrations'));
 const commands:string[]=[];page.on('request',r=>{if(r.method()==='POST')commands.push(r.url())});
 const d=drawer(page);
 await d.getByRole('button',{name:m('guided.dingtalk'),exact:true}).click();
 await d.getByLabel(m('guided.webhookUrl'),{exact:true}).fill('https://oapi.dingtalk.com/robot/send?access_token=synthetic-notification');
 await d.getByLabel(m('guided.secret'),{exact:true}).first().fill('synthetic-sign');
 await d.getByRole('button',{name:m('guided.confirmScope'),exact:true}).click();
 await expect(d.getByText(m('guided.dingtalkScope'),{exact:true})).toBeVisible();
 await expect(d.getByText(m('guided.languageNote'),{exact:true})).toBeVisible();
 await d.getByRole('button',{name:m('guided.enableDingtalk'),exact:true}).click();
 await expect(d.getByText(m('guided.dingtalkEnabled'),{exact:true})).toBeVisible();
 expect(await ownerRead(page,service,'bindings',undefined,project.id)).toHaveLength(1);
 expect(await ownerRead(page,service,'outbound-jobs',undefined,project.id)).toHaveLength(0);
 expect(commands.every(url=>url.startsWith(service.origin))).toBe(true);
});

test('advanced GitHub binding uses the saved credential reference and can be disabled',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);
 await page.goto(settings(service,project.id,'integrations'));
 const d=drawer(page);
 await d.getByText(m('advanced.title'),{exact:true}).click();
 await d.getByLabel(m('advanced.token'),{exact:true}).fill('synthetic-advanced');
 await d.getByRole('button',{name:m('advanced.saveCredential'),exact:true}).click();
 await expect(d.getByText(m('advanced.credentialSaved'),{exact:true})).toBeVisible();
 await expect(d.getByLabel(m('advanced.patRef'),{exact:true})).not.toHaveValue('');
 await d.getByLabel(m('advanced.githubRepo'),{exact:true}).fill('Owner/Advanced');
 await d.getByLabel(m('advanced.repoId'),{exact:true}).fill('77');
 await d.getByRole('button',{name:m('advanced.createGithub'),exact:true}).click();
 await expect(d.getByText(m('advanced.githubCreated'),{exact:true})).toBeVisible();
 const [binding]=await ownerRead(page,service,'bindings',undefined,project.id);
 expect(binding).toMatchObject({repository:'Owner/Advanced',repositoryId:77,private:true,enabled:true});
 await d.getByRole('button',{name:m('advanced.disable'),exact:true}).click();
 await expect(d.getByText(m('advanced.disabledNotice'),{exact:true})).toBeVisible();
 expect((await ownerRead(page,service,'bindings',undefined,project.id))[0].enabled).toBe(false);
});
