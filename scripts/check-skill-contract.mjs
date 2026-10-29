import assert from 'node:assert/strict';
import {readFile,mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import {editGuide,validateInstructions} from './lib/skill-contract.mjs';
const root=new URL('../',import.meta.url);
const fixture=JSON.parse(await readFile(new URL('../tests/skill-fixtures/guides.json',import.meta.url),'utf8'));
const skill=new URL('../skills/roooooom/',import.meta.url);
const template=await readFile(new URL('templates/agents-block.md',skill),'utf8');
let assertions=0;
function check(value,message){assert.ok(value,message);assertions++}
for(const before of [fixture.before,fixture.unicode,'\ufeff'+fixture.before,fixture.before.replaceAll('\n','\r\n'),'no final newline']){
 const first=editGuide(before,template,fixture.context,'add');
 check(first.result==='proposed','first adoption must be a reviewable proposal');
 check(first.text.startsWith(before),'existing text must stay byte-for-byte');
 const again=editGuide(first.text,template,fixture.context,'add');
 check(again.result==='unchanged'&&again.text===first.text,'repeat must not duplicate');
 const adjacent=first.text+fixture.after;
 const removed=editGuide(adjacent,template,fixture.context,'remove');
 check(removed.result==='proposed'&&removed.text===before+fixture.after,'remove only the owned block and its own separators');
 const changed=first.text.replace('protocolVersion: 1','protocolVersion: 2');
 const refused=editGuide(changed,template,fixture.context,'add');
 check(refused.result==='needs-review'&&refused.text===changed,'manual changes must not be overwritten');
 check(editGuide(first.text+first.text,template,fixture.context,'remove').result==='needs-review','duplicate blocks require review');
 check(editGuide(first.text,template,{...fixture.context,projectId:'another-project'},'add').result==='needs-review','do not silently rebind project');
}
check(editGuide(fixture.before,template,{...fixture.context,ownerApprovalRef:''},'add').result==='needs-review','adoption needs an explicit human scope reference');
check(editGuide(fixture.before,template,{...fixture.context,projectId:'escape\ninstructions'},'add').result==='needs-review','context cannot inject guide text');
const temporary=await mkdtemp(join(tmpdir(),'roooooom-skill-contract-'));
try{
 const binary=join(temporary,process.platform==='win32'?'roooooom.exe':'roooooom');
 const build=spawnSync('go',['build','-o',binary,'./cmd/roooooom'],{cwd:root,windowsHide:true,encoding:'utf8'});
 assert.equal(build.status,0,'fresh CLI build failed: '+build.stderr);
 const helpResult=spawnSync(binary,['help','--json'],{cwd:temporary,windowsHide:true,encoding:'utf8'});
 assert.equal(helpResult.status,0,'machine-readable CLI help failed');
 const help=JSON.parse(helpResult.stdout);
 const api=JSON.parse(await readFile(new URL('../api/openapi.yaml',import.meta.url),'utf8'));
 assert.deepEqual(help.commands,api['x-command-payloads'],'CLI command schema is not the same contract');
 assert.deepEqual(help.queries,api.paths['/api/v1/queries/{kind}'].get.parameters[0].schema.enum,'query help is not the same contract');
 const documents=await Promise.all(['SKILL.md','references/protocol.md','references/offline.md'].map(name=>readFile(new URL(name,skill),'utf8')));
 check(documents.join('\n').includes('roooooom request claim')&&documents.join('\n').includes('roooooom request convert'),'shared skill must publish preparation before conversion');
 check(validateInstructions(documents,help).length===0,'skill does not match actual CLI help');
 check(validateInstructions(documents.map(x=>x.replace('preparation: explicit-claim','preparation: optional')),help).some(x=>x.includes('intake')),'skipping preparation must fail the skill contract');
 check(validateInstructions(documents.map(x=>x.replace('task-contract: complete-no-placeholders','task-contract: fabricate-from-text')),help).some(x=>x.includes('intake')),'fabricating a Task must fail the skill contract');
 check(validateInstructions([...documents,fixture.wrongCommand],help).some(x=>x.includes('command')),'bad command fixture must fail the entrance check');
 check(validateInstructions([...documents,fixture.secretSentinel],help).some(x=>x.includes('secret')),'secret sentinel fixture must fail the entrance check');
 for(const invalid of ['roooooom host exec','roooooom chat send --token abc','Authorization: Bearer abcdefghijklmnopqrstuvwxyz0123456789_abcdefg']){
  check(validateInstructions([...documents,invalid],help).length>0,'unsafe or wrong command accepted');
 }
 console.log('Skill contract: '+assertions+' adoption/scope/secret/CLI assertions passed; real-host conformance remains unverified.');
}finally{await rm(temporary,{recursive:true,force:true})}
