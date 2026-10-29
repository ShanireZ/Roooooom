import {spawnSync} from 'node:child_process';
if(!process.env.npm_execpath)throw new Error('Run through npm run verify using the pinned toolchain');
const web=new URL('../',import.meta.url),root=new URL('../../',import.meta.url);
const npm=(task)=>run(process.execPath,[process.env.npm_execpath,'run',task],web);
function run(binary,args,cwd){
 console.log('Verify:',binary==='go'?'go '+args.join(' '):args.slice(1).join(' '));
 const result=spawnSync(binary,args,{cwd,windowsHide:true,stdio:'inherit'});
 if(result.status!==0)process.exit(result.status??1);
}
for(const task of ['api:check','typecheck','test:unit','build'])npm(task);
run('go',['test','./...','-count=1'],root);
run('go',['vet','./...'],root);
npm('test:e2e');
