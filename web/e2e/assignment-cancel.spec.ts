import {test,expect} from './fixture';
import {seedProject,seedTask,enroll,claimIntegrator,ownerRead,ownerCommand,taskVersions} from './helpers';
test('owner cancels one unfinished assignment with a reason and retains the task',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service),task=await seedTask(page,service,project,'integrator');
 const agent=await enroll(page,service,project.id,'integrator');await claimIntegrator(page,service,agent,task.id);
 const current=await ownerRead(page,service,'task',task.id);
 const assignment=await ownerCommand(page,service,'assignment.create',{taskId:task.id,role:'obsolete role',input:'synthetic input',output:'withdrawn output',writeScope:[],repoId:project.repositories[0].id,baseRef:'fixture-base',dependencies:[]},undefined,taskVersions(current));
 await page.goto(service.origin+'/tasks/'+task.id);
 const row=page.getByRole('article').filter({has:page.getByRole('heading',{name:'obsolete role',exact:true})});
 await row.getByRole('button',{name:'取消此分工',exact:true}).click();
 await row.getByLabel('取消分工的理由',{exact:true}).fill('synthetic obsolete responsibility');
 await row.getByRole('button',{name:'確認取消分工',exact:true}).click();
 await expect(row.getByText('已取消',{exact:true})).toBeVisible();
 await expect(row.getByText('synthetic obsolete responsibility',{exact:true})).toBeVisible();
 expect((await ownerRead(page,service,'assignment',assignment.id)).claimParticipantId??'').toBe('');
 expect((await ownerRead(page,service,'task',task.id)).phase).not.toBe('cancelled');
});
