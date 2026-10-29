import {test,expect} from './fixture';
test('closing the browser page keeps the own local service available',async({page,service,activate})=>{
 await activate();await page.close();
 const response=await fetch(service.origin+'/identity');expect(response.ok).toBe(true);expect((await response.json()).status).toBe('ok');
});
test('owner activation, refresh and deep-link use the real embedded application',async({page,service,activate})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(service.origin+'/setup');
 await expect(page.getByRole('heading',{name:'本機設定',exact:true})).toBeVisible();
 await expect(page.getByText('尚未啟用擁有者工作階段',{exact:true})).toBeVisible();
 await activate();
 await expect(page.getByText('擁有者已啟用',{exact:true})).toBeVisible();
 const cookies=await page.context().cookies();
 expect(cookies.find(c=>c.name==='roooooom_owner')).toMatchObject({httpOnly:true,sameSite:'Strict'});
 expect(await page.evaluate(()=>Object.keys(localStorage))).toEqual([]);
 await page.reload();await expect(page.getByText('擁有者已啟用',{exact:true})).toBeVisible();
 await page.goto(service.origin+'/projects');await page.reload();
 await expect(page.getByRole('heading',{name:'專案',exact:true})).toBeVisible();
 await page.screenshot({path:test.info().outputPath('desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});
 await expect(page.getByRole('heading',{name:'專案',exact:true})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 await page.screenshot({path:test.info().outputPath('mobile.png'),fullPage:true});
 await page.goto(service.origin+'/tasks/synthetic-task');await page.reload();
 await expect(page.getByRole('heading',{name:'任務詳情',exact:true})).toBeVisible();
 expect(errors).toEqual([]);
});
test('foreign Origin, project files and guessed secrets are rejected',async({page,service,activate})=>{
 await activate();
 const me=await page.request.get(service.origin+'/api/v1/me');const payload=await me.json();
 const identity=await (await page.request.get(service.origin+'/identity')).json();
 const r=await page.request.post(service.origin+'/api/v1/commands/project.create',{
  headers:{Origin:'https://foreign.invalid','X-CSRF-Token':payload.result.csrf},
  data:{serviceId:identity.serviceId,datasetGeneration:identity.datasetGeneration,clientRequestId:'foreign',payload:{name:'forbidden',repositories:[]}}
 });
 expect(r.status()).toBe(403);
 for(const path of ['/AGENTS.md','/.git/config','/credentials/owner.json','/assets/../../PLAN.md','/assets/missing.js']){
  const response=await page.request.get(service.origin+path);expect(response.status()).toBe(404);
 }
});
test('a held older query cannot suppress a fresh read after an event',async({page,service,activate})=>{
 await activate();
 let release!:()=>void,seen!:()=>void,first=true;
 const gate=new Promise<void>(r=>{release=r});
 const captured=new Promise<void>(r=>{seen=r});
 await page.route('**/api/v1/queries/projects',async route=>{
  const response=await route.fetch();
  if(first){first=false;seen();await gate}
  await route.fulfill({response});
 });
 try{
  await page.getByRole('link',{name:'專案',exact:true}).click();await captured;
  const identity=await (await page.request.get(service.origin+'/identity')).json();
  const me=await (await page.request.get(service.origin+'/api/v1/me')).json();
  const created=await page.request.post(service.origin+'/api/v1/commands/project.create',{
   headers:{Origin:service.origin,'X-CSRF-Token':me.result.csrf},
   data:{serviceId:identity.serviceId,datasetGeneration:identity.datasetGeneration,clientRequestId:crypto.randomUUID(),payload:{name:'Newer committed project',repositories:[]}}
  });
  expect(created.status()).toBe(200);
  await expect(page.getByRole('heading',{name:'Newer committed project',exact:true})).toBeVisible();
 }finally{release()}
});
