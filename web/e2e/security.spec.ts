import {test,expect} from './fixture';
import {seedProject,seedTask,enroll,claimIntegrator,agentCommand} from './helpers';
test('UnsafeMarkdownAndDisconnectedUI: messages render as text with their actual actor',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service),agent=await enroll(page,service,project.id,'Agent');
 const task=await seedTask(page,service,project);await claimIntegrator(page,service,agent,task.id);
 const body='<script data-injected>window.injected=true</script><img src=x onerror="window.injected=true">';
 await agentCommand(agent,'message.send',{projectId:project.id,taskId:task.id,threadId:'task',purpose:'discussion',priority:'normal',body,recipientMode:'participants',recipientIds:[agent.participantId],actionRequired:false});
 await page.goto(service.origin+'/tasks/'+task.id);
 await expect(page.getByText(body,{exact:true})).toBeVisible();
 await expect(page.locator('script[data-injected]')).toHaveCount(0);
 expect(await page.evaluate(()=>Reflect.get(window,'injected'))).toBeUndefined();
 await expect(page.getByTestId('message-actor')).toContainText('agent');
});
