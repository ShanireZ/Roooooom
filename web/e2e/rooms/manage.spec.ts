import {test,expect} from '../fixture';
import {claimIntegrator,enroll,ownerCommand,ownerRead,seedProject,seedTask,taskVersions} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

const drawer=(page:import('@playwright/test').Page)=>page.getByRole('dialog');

test('owner cancels one unfinished assignment with a reason and retains the task',async({page,service,activate})=>{
 await activate();
 const project=await seedProject(page,service),task=await seedTask(page,service,project,'integrator','Assignment task');
 const agent=await enroll(page,service,project.id,'Integrator');
 await claimIntegrator(page,service,agent,task.id);
 const current=await ownerRead(page,service,'task',task.id);
 const assignment=await ownerCommand(page,service,'assignment.create',{taskId:task.id,role:'obsolete role',input:'synthetic input',output:'withdrawn output',writeScope:[],repoId:project.repositories[0].id,baseRef:'fixture-base',dependencies:[]},undefined,taskVersions(current));
 await page.goto(service.origin+'/rooms/'+project.id+'/tasks/'+task.id+'/work');
 const row=drawer(page).getByRole('listitem').filter({hasText:'obsolete role'});
 await row.getByRole('button',{name:m('work.cancel')}).click();
 await row.getByLabel(m('work.cancelReason')).fill('synthetic obsolete responsibility');
 await row.getByRole('button',{name:m('work.confirmCancel')}).click();
 await expect(row).toContainText(m('assignment.state.cancelled'));
 await expect(row).toContainText('synthetic obsolete responsibility');
 expect((await ownerRead(page,service,'assignment',assignment.id)).claimParticipantId??'').toBe('');
 expect((await ownerRead(page,service,'task',task.id)).phase).not.toBe('cancelled');
});

test('collaborator limit and a forced integrator handoff are explicit owner actions',async({page,service,activate})=>{
 await activate();
 const project=await seedProject(page,service),task=await seedTask(page,service,project,'integrator','Handoff task');
 const a=await enroll(page,service,project.id,'First integrator'),b=await enroll(page,service,project.id,'Next integrator');
 await claimIntegrator(page,service,a,task.id);
 await page.goto(service.origin+'/rooms/'+project.id+'/tasks/'+task.id+'/manage');
 await drawer(page).getByText(m('manage.limit'),{exact:true}).click();
 await drawer(page).getByLabel(m('manage.newLimit')).fill('3');
 await drawer(page).getByLabel(m('manage.limitReason')).fill('more help');
 await drawer(page).getByRole('button',{name:m('manage.updateLimit')}).click();
 await expect.poll(async()=>(await ownerRead(page,service,'task',task.id)).collaboratorLimit).toBe(3);
 await drawer(page).getByText(m('manage.handoff'),{exact:true}).click();
 await drawer(page).getByLabel(m('manage.handoffTarget')).selectOption({index:1});
 await drawer(page).getByLabel(m('manage.handoffReason')).fill('first integrator stopped');
 await drawer(page).getByLabel(m('manage.handoffConfirm')).check();
 await drawer(page).getByRole('button',{name:m('manage.confirmHandoff')}).click();
 await expect.poll(async()=>(await ownerRead(page,service,'task',task.id)).integratorId).toBe(b.participantId);
});

test('an open request takeover form keeps its original version and offers named sessions',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),a=await enroll(page,service,p.id,'Candidate preparer');
 const r=await ownerCommand(page,service,'request.create',{projectId:p.id,text:'Old screens must not overwrite new responsibility'});
 await page.goto(service.origin+'/rooms/'+p.id+'/requests/'+r.id);
 await drawer(page).getByText(m('request.manage')).click();
 await drawer(page).getByLabel(m('request.reason')).fill('handle the original version');
 await ownerCommand(page,service,'request.assign',{reason:'assigned elsewhere',toSessionId:a.sessionId,expectedSourceSessionId:''},r.id,{expectedVersion:r.version,claimEpoch:r.preparationEpoch});
 await expect(drawer(page)).toContainText('Candidate preparer');
 await expect(drawer(page).getByRole('button',{name:m('request.confirmAssign')})).toBeDisabled();
 await drawer(page).getByRole('button',{name:m('form.reopen')}).click();
 await drawer(page).getByLabel(m('request.reason')).fill('I cancel this version and keep the text.');
 await drawer(page).getByRole('button',{name:m('request.confirmCancel')}).click();
 await expect(drawer(page)).toContainText(m('request.state.cancelled'));
 expect((await ownerRead(page,service,'request',r.id)).text).toBe(r.text);
});

test('designated integrator is distinct from preparation ownership',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),a=await enroll(page,service,p.id,'Integrator A');
 await enroll(page,service,p.id,'Preparer B');
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:m('room.ask')}).click();
 await page.getByLabel(m('composer.askLabel')).fill('Qualified members prepare; A integrates');
 await page.getByText(m('composer.more')).click();
 await page.getByLabel(m('composer.integrator')).selectOption(a.participantId);
 await expect(page.getByText(m('composer.integratorHint'))).toBeVisible();
 await page.getByRole('button',{name:m('composer.askSubmit')}).click();
 await expect(page.getByRole('list',{name:m('room.door')})).toContainText('Qualified members prepare');
 const [request]=await ownerRead(page,service,'requests',undefined,p.id);
 expect(request.designatedIntegratorId).toBe(a.participantId);
 expect(request.preparerParticipantId??'').toBe('');
});

test('the full task editor distinguishes same-name integrators and acceptors',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),a=await enroll(page,service,p.id,'Same-name agent'),b=await enroll(page,service,p.id,'Same-name agent');
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:m('composer.fullSpec')}).click();
 const editor=page.getByRole('dialog',{name:m('create.title')});
 await editor.getByLabel(m('create.goal'),{exact:true}).fill('Tell responsibilities apart');
 await editor.getByLabel(m('create.scope'),{exact:true}).fill('synthetic test');
 await editor.getByLabel(m('create.authority'),{exact:true}).fill('synthetic://owner');
 await editor.getByLabel(m('composer.integrator')).selectOption({label:'Same-name agent · '+a.participantId.slice(0,8)});
 await editor.getByLabel(m('create.acceptanceMode'),{exact:true}).selectOption('designated-agent');
 await editor.getByLabel(m('create.acceptor')).selectOption({label:'Same-name agent · '+b.participantId.slice(0,8)});
 await editor.getByLabel(m('create.conditions'),{exact:true}).fill('tests pass');
 await editor.getByRole('button',{name:m('create.submit')}).click();
 await expect(page).toHaveURL(/\/rooms\/[^/]+\/tasks\//);
 const task=await ownerRead(page,service,'task',page.url().split('/').pop());
 expect(task.designatedIntegratorId).toBe(a.participantId);
 expect(task.acceptancePolicy.acceptorId).toBe(b.participantId);
});

test('a stale manage form disables with Reopen',async({page,service,activate})=>{
 await activate();
 const project=await seedProject(page,service),task=await seedTask(page,service,project,'integrator','Stale task');
 await page.goto(service.origin+'/rooms/'+project.id+'/tasks/'+task.id+'/manage');
 await drawer(page).getByText(m('manage.limit'),{exact:true}).click();
 const current=await ownerRead(page,service,'task',task.id);
 await ownerCommand(page,service,'collaborator.limit',{limit:4,reason:'changed elsewhere'},task.id,taskVersions(current));
 await expect(drawer(page).getByText(m('form.stale'))).toBeVisible();
 await expect(drawer(page).getByRole('button',{name:m('manage.updateLimit')})).toBeDisabled();
 await drawer(page).getByRole('button',{name:m('form.reopen')}).click();
 await expect(drawer(page).getByRole('button',{name:m('manage.updateLimit')})).toBeEnabled();
});

test('destructive actions need a reason, an explicit confirmation and danger styling',async({page,service,activate})=>{
 await activate();
 const project=await seedProject(page,service),task=await seedTask(page,service,project,'integrator','Cancel task');
 await page.goto(service.origin+'/rooms/'+project.id+'/tasks/'+task.id+'/manage');
 await drawer(page).getByRole('button',{name:m('manage.cancel')}).click();
 const confirm=drawer(page).getByRole('button',{name:m('manage.confirmCancel')});
 await expect(confirm).toHaveClass(/btn-danger/);
 await confirm.click();
 expect((await ownerRead(page,service,'task',task.id)).phase).not.toBe('cancelled');
 await drawer(page).getByLabel(m('manage.cancelReason')).fill('not needed');
 await confirm.click();
 await expect.poll(async()=>(await ownerRead(page,service,'task',task.id)).phase).toBe('cancelled');
});
