import {test as base,expect} from '@playwright/test';
import {spawn, type ChildProcess} from 'node:child_process';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {pathToFileURL} from 'node:url';
type LocalService={root:string;origin:string;stop:()=>Promise<void>};
export const test=base.extend<{service:LocalService;activate:()=>Promise<void>}>({
 service:async({},use)=>{
  const root=await mkdtemp(join(tmpdir(),'roooooom-e2e-private-'));
  const child:ChildProcess=spawn(process.env.ROOOOOOM_E2E_BINARY!,['serve','--data-dir',root,'--listen','127.0.0.1:0'],{windowsHide:true,stdio:'ignore'});
  let stopped=false;
  const stop=async()=>{
   if(stopped)return;stopped=true;
   if(child.exitCode===null && child.signalCode===null){const exited=new Promise<void>(r=>child.once('exit',()=>r()));child.kill();await exited}
  };
  try{
   let origin='';
   for(let i=0;i<100;i++){
    try{const d=JSON.parse(await readFile(join(root,'discovery.json'),'utf8'));origin='http://'+d.listenAddr;if((await fetch(origin+'/health')).ok)break}catch{}
    await new Promise(r=>setTimeout(r,50));
   }
   if(!origin)throw new Error('isolated Go service failed to start');
   await use({root,origin,stop});
  }finally{await stop();await rm(root,{recursive:true,force:true})}
 },
 activate:async({page,service},use)=>{
  await use(async()=>{
   await page.goto(pathToFileURL(join(service.root,'activation.html')).href);
   await page.getByRole('button',{name:'開始使用',exact:true}).click();
   await expect(page).toHaveURL(service.origin+'/setup');
  });
 }
});
export {expect};
