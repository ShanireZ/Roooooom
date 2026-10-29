import {expect,type Page} from '@playwright/test';
import {readFile} from 'node:fs/promises';
export type Service={root:string;origin:string};
export type Agent={participantId:string;sessionId:string;token:string;service:Service;identity:any};
export async function ownerCommand(page:Page,service:Service,kind:string,payload:any,targetId?:string,versions:any={}){
 const identity=await (await page.request.get(service.origin+'/identity')).json();
 const me=await (await page.request.get(service.origin+'/api/v1/me')).json();
 const response=await page.request.post(service.origin+'/api/v1/commands/'+kind,{
  headers:{Origin:service.origin,'X-CSRF-Token':me.result.csrf},
  data:{...versions,serviceId:identity.serviceId,datasetGeneration:identity.datasetGeneration,clientRequestId:crypto.randomUUID(),targetId,payload}
 });
 const data=await response.json();
 expect(response.ok(),kind+': '+(data.error?.code??response.status())).toBe(true);
 return data.result;
}
export async function ownerRead(page:Page,service:Service,kind:string,targetId?:string,projectId?:string){
 const params=new URLSearchParams();if(targetId)params.set('targetId',targetId);if(projectId)params.set('projectId',projectId);
 const response=await page.request.get(service.origin+'/api/v1/queries/'+kind+'?'+params);
 expect(response.ok()).toBe(true);return (await response.json()).result;
}
export async function seedProject(page:Page,service:Service){
 return ownerCommand(page,service,'project.create',{name:'Protocol rehearsal',repositories:[{name:'fixture-repo',reference:'synthetic://fixture'}]});
}
export async function enroll(page:Page,service:Service,project:string,name:string):Promise<Agent>{
 const grant=await ownerCommand(page,service,'enrollment.enable',{},project);
 const capability=JSON.parse(await readFile(grant.enrollmentPath,'utf8')).capability;
 const identity=await (await page.request.get(service.origin+'/identity')).json();
 const response=await fetch(service.origin+'/api/v1/enroll',{method:'POST',headers:{Authorization:'Bearer '+capability,'Content-Type':'application/json'},body:JSON.stringify({serviceId:identity.serviceId,datasetGeneration:identity.datasetGeneration,clientRequestId:crypto.randomUUID(),name})});
 expect(response.ok).toBe(true);return {...(await response.json()).result,service,identity};
}
export async function agentCommand(agent:Agent,kind:string,payload:any,targetId?:string,versions:any={}){
 const response=await fetch(agent.service.origin+'/api/v1/commands/'+kind,{method:'POST',headers:{Authorization:'Bearer '+agent.token,'Content-Type':'application/json'},body:JSON.stringify({...versions,serviceId:agent.identity.serviceId,datasetGeneration:agent.identity.datasetGeneration,clientRequestId:crypto.randomUUID(),sessionId:agent.sessionId,targetId,payload})});
 const data=await response.json();expect(response.ok,kind+': '+(data.error?.code??response.status)).toBe(true);return data.result;
}
export function taskVersions(task:any){return {expectedVersion:task.version,integratorEpoch:task.integratorEpoch,specVersion:task.specVersion,acceptancePolicyVersion:task.acceptancePolicyVersion,decisionVersions:task.decisionVersions??{}}}
export async function seedTask(page:Page,service:Service,project:any,mode='human',goal='Synthetic collaborative change'){
 return ownerCommand(page,service,'task.create',{projectId:project.id,goal,scope:['fixture only'],nonScope:['publication'],authorityRefs:['synthetic://owner'],risk:'normal',acceptancePolicy:{mode,conditions:['tests on exact head']},collaboratorLimit:1});
}
export async function claimIntegrator(page:Page,service:Service,agent:Agent,taskId:string){
 const task=await ownerRead(page,service,'task',taskId);
 return agentCommand(agent,'integrator.claim',{},taskId,taskVersions(task));
}
export async function makeCandidate(page:Page,service:Service,project:any,taskId:string,agents:Agent[]){
 let assignments=await ownerRead(page,service,'assignments',taskId);
 if(!assignments.length){
  for(let i=0;i<agents.length;i++){
   const task=await ownerRead(page,service,'task',taskId);
   await ownerCommand(page,service,'assignment.create',{taskId,role:i===0?'core':'web',input:'approved fixture contract',output:'commit and checks',writeScope:[i===0?'core/':'web/'],repoId:project.repositories[0].id,baseRef:'synthetic-base',dependencies:[]},undefined,taskVersions(task));
  }
  assignments=await ownerRead(page,service,'assignments',taskId);
 }
 const deliveries:string[]=[];
 for(let i=0;i<assignments.length;i++){
  let assignment=assignments[i],task=await ownerRead(page,service,'task',taskId);
  const actor=agents[i%agents.length];
  await agentCommand(actor,'assignment.claim',{},assignment.id,{...taskVersions(task),expectedVersion:assignment.version});
  assignment=await ownerRead(page,service,'assignment',assignment.id);task=await ownerRead(page,service,'task',taskId);
  const delivery=await agentCommand(actor,'delivery.submit',{repoId:project.repositories[0].id,baseSha:'0'.repeat(40),headSha:String(i+1).repeat(40),branchRef:'synthetic/worker-'+i,workspaceRef:'synthetic://worktree/'+i,summary:'Synthetic protocol evidence',evidenceRefs:['synthetic://check'],unknowns:[],artifactRefs:[]},assignment.id,{...taskVersions(task),expectedVersion:assignment.version,claimEpoch:assignment.claimEpoch});
  deliveries.push(delivery.id);
  assignment=await ownerRead(page,service,'assignment',assignment.id);task=await ownerRead(page,service,'task',taskId);
  await agentCommand(agents[0],'delivery.review',{deliveryRevisionId:delivery.id,disposition:'accepted',reason:'Synthetic fixture review'},assignment.id,{...taskVersions(task),expectedVersion:assignment.version});
 }
 let task=await ownerRead(page,service,'task',taskId);
 const repo=project.repositories[0].id,head='3'.repeat(40);
 const integration=await agentCommand(agents[0],'integration.record',{deliveryRevisionIds:deliveries,heads:[{repoId:repo,baseSha:'0'.repeat(40),headSha:head}],checks:task.acceptancePolicy.conditions.map((condition:string)=>({condition,outcome:'passed',headShas:{[repo]:head},evidenceRefs:['synthetic://integration-check']})),summary:'Synthetic integrated result',result:'passed',excludedAssignments:[],untested:[]},taskId,taskVersions(task));
 task=await ownerRead(page,service,'task',taskId);
 return agentCommand(agents[0],'acceptance.candidate',{integrationId:integration.id},taskId,taskVersions(task));
}
export async function askDecision(page:Page,service:Service,agent:Agent,taskId:string,actions=['implementation']){
 const task=await ownerRead(page,service,'task',taskId);
 return agentCommand(agent,'decision.request',{taskId,scopeKey:'fixture-choice',question:'要採用哪個資料格式？',reason:'共同介面需要確認',factRefs:['synthetic://contract'],options:[{id:'a',label:'採用方案 A',impact:'維持現有資料',outcome:'approved'},{id:'b',label:'暫緩變更',impact:'等待補充證據',outcome:'revise'}],recommendation:'a',recommendationReason:'相容性較好',impact:'影響共同介面',blockedAssignmentIds:[],blockedActions:actions,canContinue:['獨立文件整理'],noDecisionBehavior:'保持阻塞'},undefined,taskVersions(task));
}
