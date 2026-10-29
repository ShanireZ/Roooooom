import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile,mkdtemp,writeFile,mkdir,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';

const checker=new URL('./check-publication.mjs',import.meta.url);
function run(cwd,command,args){return spawnSync(command,args,{cwd,encoding:'utf8',windowsHide:true})}
const required=['README.md','README.zh-CN.md','README.zh-TW.md','README.ja.md','LICENSE','COPYRIGHT'];
test('public gate audits staged bytes, English docs and GPL metadata',async()=>{
 const root=await mkdtemp(join(tmpdir(),'roooooom-publication-20261029-'));
 try{
  assert.equal(run(root,'git',['init','--initial-branch=main']).status,0);
  for(const file of required)await writeFile(join(root,file),file==='LICENSE'?'GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n':'English public documentation\n');
  await writeFile(join(root,'.gitignore'),'.local/\nAGENTS.md\nPLAN.md\n.env\n');
  await mkdir(join(root,'web'));await mkdir(join(root,'packaging'));
  await writeFile(join(root,'web/package.json'),JSON.stringify({license:'GPL-3.0-only'}));await writeFile(join(root,'packaging/platforms.json'),JSON.stringify({license:'GPL-3.0-only'}));
  run(root,'git',['add','.']);let result=run(root,process.execPath,[checker.pathname.replace(/^\/([A-Za-z]:)/,'$1'),'--root',root]);assert.equal(result.status,0,result.stderr+result.stdout);
  await writeFile(join(root,'PLAN.md'),'Private planning\n');run(root,'git',['add','--force','PLAN.md']);result=run(root,process.execPath,[checker.pathname.replace(/^\/([A-Za-z]:)/,'$1'),'--root',root]);assert.notEqual(result.status,0);assert.match(result.stdout+result.stderr,/private-path/);
  run(root,'git',['rm','--cached','PLAN.md']);await writeFile(join(root,'DESIGN.md'),'\u79c1\u6709\u8cc7\u6599');run(root,'git',['add','DESIGN.md']);result=run(root,process.execPath,[checker.pathname.replace(/^\/([A-Za-z]:)/,'$1'),'--root',root]);assert.notEqual(result.status,0);assert.match(result.stdout+result.stderr,/english-doc/);
  await writeFile(join(root,'DESIGN.md'),'English staged content\n');run(root,'git',['add','DESIGN.md']);await writeFile(join(root,'DESIGN.md'),'ghp_'+'x'.repeat(36));run(root,'git',['add','DESIGN.md']);await writeFile(join(root,'DESIGN.md'),'Clean working tree cannot hide staged secret\n');result=run(root,process.execPath,[checker.pathname.replace(/^\/([A-Za-z]:)/,'$1'),'--root',root]);assert.notEqual(result.status,0);assert.match(result.stdout+result.stderr,/secret-like/);
 }finally{await rm(root,{recursive:true,force:true})}
});
test('project package metadata selects the exact GPL variant',async()=>{
 const web=JSON.parse(await readFile(new URL('../web/package.json',import.meta.url),'utf8'));const platforms=JSON.parse(await readFile(new URL('../packaging/platforms.json',import.meta.url),'utf8'));assert.equal(web.license,'GPL-3.0-only');assert.equal(platforms.license,'GPL-3.0-only');
});
