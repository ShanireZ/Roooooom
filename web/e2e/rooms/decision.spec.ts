import {test,expect} from '../fixture';
import {agentCommand,askDecision,claimIntegrator,enroll,ownerCommand,ownerRead,seedProject,seedTask,taskVersions} from '../helpers';
import {m} from '../i18n';

test.use({locale:'en-US'});
const confirm=(page:import('@playwright/test').Page)=>page.getByRole('dialog').getByRole('button',{name:new RegExp('^'+m('decision.confirmPrefix'))});

async function taskQuestion(page:any,service:any,actions=['implementation']){
 const p=await seedProject(page,service),agent=await enroll(page,service,p.id,'Integrator');
 const task=await seedTask(page,service,p,'integrator','Question task');
 await claimIntegrator(page,service,agent,task.id);
 const question=await askDecision(page,service,agent,task.id,actions);
 return {p,agent,task,question};
}

test('a request-scoped option decision needs no duplicated answer and keeps its identity',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),a=await enroll(page,service,p.id,'Preparer');
 let intake=await ownerCommand(page,service,'request.create',{projectId:p.id,text:'Confirm the format before creating a task'});
 await agentCommand(a,'request.claim',{},intake.id,{expectedVersion:intake.version,claimEpoch:0});
 intake=await ownerRead(page,service,'request',intake.id);
 const question=await agentCommand(a,'decision.request',{requestId:intake.id,scopeKey:'format',question:'Which report format?',reason:'A choice is needed',factRefs:['synthetic://format'],options:[{id:'a',label:'Use integers',impact:'easy to read',outcome:'approved'},{id:'b',label:'Keep decimals',impact:'more precise',outcome:'answered'}],recommendation:'a',recommendationReason:'good enough',impact:'display format',blockedActions:['request-convert'],blockedAssignmentIds:[],canContinue:['read authorities'],noDecisionBehavior:'wait'},undefined,{expectedVersion:intake.version,claimEpoch:intake.preparationEpoch});
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 const option=page.getByRole('radio',{name:/Use integers/});
 await expect(option).not.toBeChecked();
 await option.check();
 await expect(confirm(page)).toHaveText(m('decision.confirmPrefix')+'Use integers');
 await confirm(page).click();
 await expect(page.getByTestId('formal-decision')).toContainText('Use integers');
 expect((await ownerRead(page,service,'decision',question.id)).request.requestId).toBe(intake.id);
 expect(await ownerRead(page,service,'tasks')).toHaveLength(0);
});

test('a prepared external action needs a separate explicit scoped confirmation',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service),a=await enroll(page,service,p.id,'Integrator'),task=await seedTask(page,service,p);
 await claimIntegrator(page,service,a,task.id);
 const current=await ownerRead(page,service,'task',task.id);
 const proposal={action:'push',target:'synthetic://origin/named-topic',scope:'push only the named-topic branch',originalText:'confirm one push in this scope',expiresAt:new Date(Date.now()+3600000).toISOString(),maxUses:1};
 const question=await agentCommand(a,'decision.request',{taskId:task.id,scopeKey:'publish',question:'Authorize this push?',reason:'explicit authorization needed',factRefs:['synthetic://review'],options:[{id:'yes',label:'Confirm the push',impact:'named branch only',outcome:'approved'},{id:'no',label:'Do not push',impact:'keep local',outcome:'rejected'}],recommendation:'no',recommendationReason:'wait',impact:'external action',blockedActions:['external-action'],blockedAssignmentIds:[],canContinue:['read-only checks'],noDecisionBehavior:'nothing happens',externalProposal:proposal},undefined,taskVersions(current));
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 const drawer=page.getByRole('dialog');
 await expect(drawer).toContainText(proposal.target);
 await expect(drawer).toContainText(m('decision.notApproved'));
 await page.getByRole('radio',{name:/Confirm the push/}).check();
 const toggle=drawer.getByLabel(m('decision.authorizeExternal'));
 await expect(toggle).not.toBeChecked();
 await toggle.check();
 await expect(drawer.getByLabel(m('decision.externalTarget'))).toHaveValue(proposal.target);
 await expect(drawer.getByLabel(m('decision.externalScope'))).toHaveValue(proposal.scope);
 await confirm(page).click();
 await expect(page.getByTestId('formal-decision')).toBeVisible();
 const history=(await ownerRead(page,service,'decision',question.id)).history;
 expect(history[0].externalAuthorization).toMatchObject({target:proposal.target,scope:proposal.scope,action:'push',maxUses:1});
});

test('custom answers and withdrawal need text; a recommendation never submits itself',async({page,service,activate})=>{
 await activate();
 const {p,question}=await taskQuestion(page,service);
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 await expect(confirm(page)).toBeDisabled();
 await page.getByRole('radio',{name:m('decision.other')}).check();
 await confirm(page).click();
 expect((await ownerRead(page,service,'decision',question.id)).history).toHaveLength(0);
 await page.getByRole('button',{name:m('decision.withdraw')}).click();
 await page.getByRole('button',{name:m('decision.confirmWithdraw')}).click();
 expect((await ownerRead(page,service,'decision',question.id)).request.state).toBe('pending');
 await page.getByLabel(m('decision.withdrawReason')).fill('The question no longer applies.');
 await page.getByRole('button',{name:m('decision.confirmWithdraw')}).click();
 await expect(page.getByText(m('decision.withdrawn'))).toBeVisible();
 expect((await ownerRead(page,service,'decision',question.id)).request.state).toBe('withdrawn');
});

test('an agent relay is chat; only the owner page records the formal answer',async({page,service,activate})=>{
 await activate();
 const {p,agent,task,question}=await taskQuestion(page,service);
 await agentCommand(agent,'message.send',{projectId:p.id,taskId:task.id,threadId:'task',purpose:'decision-notice',priority:'normal',body:'Owner approved: choose A',recipientMode:'participants',recipientIds:[agent.participantId],actionRequired:false});
 expect((await ownerRead(page,service,'decision',question.id)).history).toHaveLength(0);
 const rejected=await fetch(service.origin+'/api/v1/commands/decision.resolve',{method:'POST',headers:{Authorization:'Bearer '+agent.token,'Content-Type':'application/json'},body:'{}'});
 expect(rejected.status).toBe(403);
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 await expect(page.getByTestId('formal-decision')).toHaveCount(0);
 await page.getByRole('radio',{name:/採用方案 A/}).check();
 await page.getByText(m('decision.supplement')).click();
 await page.getByLabel(m('decision.supplementLabel')).fill('Confirmed, within the current scope.');
 await confirm(page).click();
 await expect(page.getByText(m('decision.saved'))).toBeVisible();
 const history=(await ownerRead(page,service,'decision',question.id)).history;
 expect(history).toHaveLength(1);
 expect(history[0].actorKind).toBe('human');
 expect(history[0].answer).toContain('Confirmed, within the current scope.');
});

test('a spec amendment, authority follow-up and a revision keep distinct versions',async({page,service,activate})=>{
 await activate();
 const {p,agent,task,question}=await taskQuestion(page,service,['implementation','spec-change']);
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 const drawer=page.getByRole('dialog');
 const amend=drawer.getByLabel(m('decision.amendSpec'));
 await expect(amend).toBeVisible();
 await expect(amend).not.toBeChecked();
 await page.getByRole('radio',{name:/採用方案 A/}).check();
 await drawer.getByText(m('decision.basis')).click();
 await drawer.getByLabel(m('decision.followup')).check();
 await amend.check();
 await drawer.getByLabel(m('decision.amendGoal')).fill('Revised owner scope');
 await confirm(page).click();
 await expect(page.getByText(m('decision.saved'))).toBeVisible();
 let current=await ownerRead(page,service,'task',task.id);
 expect(current.specVersion).toBe(2);
 expect(current.goal).toBe('Revised owner scope');
 const detail=await ownerRead(page,service,'decision',question.id);
 expect(detail.request.followUpRequired).toBe(true);
 await agentCommand(agent,'decision.followup',{reason:'authority updated',evidenceRefs:['synthetic://authority-v2']},question.id,{...taskVersions(current),expectedVersion:detail.request.version});
 await page.getByRole('button',{name:m('decision.revise')}).click();
 await page.getByRole('radio',{name:/暫緩變更/}).check();
 await confirm(page).click();
 await expect.poll(async()=>(await ownerRead(page,service,'decision',question.id)).history.length).toBe(2);
});

test('selecting an option records nothing until the labelled confirm button is pressed',async({page,service,activate})=>{
 await activate();
 const {p,question}=await taskQuestion(page,service);
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 await page.getByRole('radio',{name:/採用方案 A/}).check();
 await expect(confirm(page)).toHaveText(m('decision.confirmPrefix')+'採用方案 A');
 await page.waitForTimeout(500);
 expect((await ownerRead(page,service,'decision',question.id)).history).toHaveLength(0);
});

test('Next item moves to the next room’s item after resolving',async({page,service,activate})=>{
 await activate();
 const first=await taskQuestion(page,service);
 const second=await taskQuestion(page,service);
 await page.goto(service.origin+'/rooms/'+first.p.id+'/decisions/'+first.question.id);
 await page.getByRole('radio',{name:/採用方案 A/}).check();
 await confirm(page).click();
 await page.getByRole('button',{name:m('decision.next')}).click();
 await expect(page).toHaveURL(service.origin+'/rooms/'+second.p.id+'/decisions/'+second.question.id);
});

test('the system risk question renders in the interface language',async({page,service,activate})=>{
 await activate();
 const p=await seedProject(page,service);
 const task=await ownerCommand(page,service,'task.create',{projectId:p.id,goal:'Risky change',scope:['fixture'],nonScope:[],authorityRefs:['synthetic://owner'],risk:'high',acceptancePolicy:{mode:'integrator',conditions:['tests pass']},collaboratorLimit:0});
 const [question]=await ownerRead(page,service,'decisions',task.id);
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 await expect(page.getByRole('heading',{level:2,name:m('sysq.risk.question')})).toBeVisible();
 await expect(page.getByRole('radio',{name:new RegExp(m('sysq.risk.option.authorize-bounded.label'))})).toBeVisible();
});

test('a question changed elsewhere disables the open form until it is reopened',async({page,service,activate})=>{
 await activate();
 const {p,question}=await taskQuestion(page,service);
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 await page.getByRole('radio',{name:/採用方案 A/}).check();
 await ownerCommand(page,service,'decision.resolve',{answer:'answered on another screen',optionId:'b'},question.id,{expectedVersion:question.version});
 await expect(page.getByRole('dialog').getByText(m('form.stale'))).toBeVisible();
 await expect(confirm(page)).toBeDisabled();
});

test('a lost confirm response keeps the selection and an explicit retry resolves exactly once',async({page,service,activate})=>{
 await activate();
 const {p,question}=await taskQuestion(page,service);
 await page.goto(service.origin+'/rooms/'+p.id+'/decisions/'+question.id);
 let lost=true;const bodies:unknown[]=[];
 await page.route('**/commands/decision.resolve',async route=>{bodies.push(route.request().postDataJSON());if(lost){lost=false;await route.fetch();await route.abort('failed')}else await route.continue()});
 await page.getByRole('radio',{name:/採用方案 A/}).check();
 await page.getByText(m('decision.supplement')).click();
 await page.getByLabel(m('decision.supplementLabel')).fill('kept note');
 await confirm(page).click();
 await expect(page.getByRole('dialog').getByRole('alert')).toBeVisible();
 await expect(page.getByRole('radio',{name:/採用方案 A/})).toBeChecked();
 await expect(page.getByLabel(m('decision.supplementLabel'))).toHaveValue('kept note');
 const retry=page.getByRole('dialog').getByRole('button',{name:m('decision.retry')});
 await expect(retry).toBeEnabled({timeout:10000});
 await retry.click();
 await expect(page.getByText(m('decision.saved'))).toBeVisible();
 expect(bodies).toHaveLength(2);
 expect(bodies[0]).toEqual(bodies[1]);
 expect((await ownerRead(page,service,'decision',question.id)).history).toHaveLength(1);
});
