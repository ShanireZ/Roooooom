import {test,expect} from '../fixture';
import {agentCommand,claimIntegrator,enroll,makeCandidate,ownerCommand,ownerRead,seedProject,seedTask,taskVersions} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});

async function candidateTask(page:any,service:any,goal='Acceptance goal'){
 const p=await seedProject(page,service),a=await enroll(page,service,p.id,'Codex');
 const task=await seedTask(page,service,p,'human',goal);
 await claimIntegrator(page,service,a,task.id);
 const candidate=await makeCandidate(page,service,p,task.id,[a]);
 return {p,a,task,candidate};
}

test('acceptance is offered only after a valid candidate exists',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),task=await seedTask(page,service,p,'human','Wait for a real candidate');
 await page.goto(service.origin+'/rooms/'+p.id);
 await page.getByRole('button',{name:new RegExp(m('shell.needsYou'))}).click();
 await expect(page.getByText(m('needsYou.empty'))).toBeVisible();
 await page.keyboard.press('Escape');
 const a=await enroll(page,service,p.id,'Codex');
 await claimIntegrator(page,service,a,task.id);
 await makeCandidate(page,service,p,task.id,[a]);
 await page.goto(service.origin+'/rooms/'+p.id+'/acceptance/'+task.id);
 await expect(page.getByRole('button',{name:m('acceptance.accept')})).toBeEnabled();
});

test('conditions show as a checklist with outcomes; technical details hold heads and the candidate',async({page,service,activate})=>{
 await activate();
 const {p,task,candidate}=await candidateTask(page,service);
 await page.goto(service.origin+'/rooms/'+p.id+'/acceptance/'+task.id);
 const drawer=page.getByRole('dialog');
 const checklist=drawer.getByRole('list',{name:m('acceptance.conditions')});
 await expect(checklist.getByRole('listitem')).toHaveCount(task.acceptancePolicy.conditions.length);
 await expect(checklist).toContainText(task.acceptancePolicy.conditions[0]);
 await expect(drawer).toContainText(m('acceptance.reportedBy',{name:'Codex'}));
 await expect(drawer.getByText(candidate.id)).toBeHidden();
 await drawer.getByText(m('fold.technical')).click();
 await expect(drawer.getByText(candidate.id,{exact:false})).toBeVisible();
 await expect(drawer).toContainText('3'.repeat(40));
 await drawer.getByRole('button',{name:m('acceptance.accept')}).click();
 await expect(drawer).toContainText(m('acceptance.accepted'));
 const accepted=await ownerRead(page,service,'acceptances',task.id);
 expect(accepted.some((x:any)=>x.candidateId===candidate.id&&x.actorKind==='human')).toBe(true);
});

test('commenting opens the task discussion and does not reject',async({page,service,activate})=>{
 await activate();
 const {p,task}=await candidateTask(page,service);
 await page.goto(service.origin+'/rooms/'+p.id+'/acceptance/'+task.id);
 await page.getByRole('button',{name:m('acceptance.comment')}).click();
 await expect(page).toHaveURL(service.origin+'/rooms/'+p.id+'/tasks/'+task.id+'/discussion');
 expect(await ownerRead(page,service,'acceptances',task.id)).toHaveLength(0);
});

test('a stale candidate shows its status instead of the accept button',async({page,service,activate})=>{
 await activate();
 const {p,task}=await candidateTask(page,service);
 const current=await ownerRead(page,service,'task',task.id);
 const policy={...current.acceptancePolicy,conditions:[...current.acceptancePolicy.conditions,'a new owner requirement']};
 await ownerCommand(page,service,'acceptance.policy',{policy,reason:'new requirement'},task.id,taskVersions(current));
 await page.goto(service.origin+'/rooms/'+p.id+'/acceptance/'+task.id);
 await expect(page.getByRole('dialog')).toContainText(m('acceptance.none'));
 await expect(page.getByRole('button',{name:m('acceptance.accept')})).toHaveCount(0);
});

test('archive search and status filter find completed work among many records',async({page,service,activate})=>{
 await activate();
 const {p,a,task,candidate}=await candidateTask(page,service,'Unique finished report');
 let current=await ownerRead(page,service,'task',task.id);
 await ownerCommand(page,service,'acceptance.accept',{candidateId:candidate.id},task.id,taskVersions(current));
 current=await ownerRead(page,service,'task',task.id);
 await agentCommand(a,'task.complete',{candidateId:candidate.id},task.id,taskVersions(current));
 for(let i=0;i<40;i++){
  const t=await seedTask(page,service,p,'integrator','Cancelled filler '+i);
  const fresh=await ownerRead(page,service,'task',t.id);
  await ownerCommand(page,service,'task.cancel',{reason:'filler'},t.id,taskVersions(fresh));
 }
 await page.goto(service.origin+'/rooms/'+p.id);
 await expect(page.getByRole('link',{name:new RegExp(m('room.cabinet'))})).toContainText('1');
 await page.getByRole('link',{name:new RegExp(m('room.cabinet'))}).click();
 const drawer=page.getByRole('dialog',{name:m('archive.title')});
 await drawer.getByLabel(m('archive.search')).fill('unique finished');
 await expect(drawer.getByRole('list',{name:m('archive.results')}).getByRole('listitem')).toHaveCount(1);
 await expect(drawer).toContainText('Unique finished report');
 await drawer.getByLabel(m('archive.search')).fill('');
 await drawer.getByLabel(m('archive.status')).selectOption('cancelled');
 await expect(drawer.getByRole('list',{name:m('archive.results')}).getByRole('listitem')).toHaveCount(40);
 await drawer.getByLabel(m('archive.status')).selectOption('done');
 await drawer.getByRole('link',{name:/Unique finished report/}).click();
 await expect(page).toHaveURL(service.origin+'/rooms/'+p.id+'/tasks/'+task.id);
});
