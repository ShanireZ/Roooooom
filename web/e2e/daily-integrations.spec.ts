import {test,expect} from './fixture';
import {seedProject,ownerRead} from './helpers';

test('failed metadata inspection resumes without storing the credential twice',async({page,service,activate})=>{
 await activate();await seedProject(page,service);await page.goto(service.origin+'/settings');let credentials=0,inspections=0;
 page.on('request',r=>{if(r.url().endsWith('/binding.credential'))credentials++});
 await page.route('**/commands/binding.inspect',async route=>{inspections++;await route.fulfill({status:inspections===1?409:200,contentType:'application/json',body:JSON.stringify(inspections===1?{error:{code:'github-metadata-unavailable'}}:{result:{repository:'Owner/Repo',repositoryId:42,private:false,hasIssues:true,issuesWriteVerified:false}})})});
 await page.getByRole('button',{name:'連接 GitHub',exact:true}).click();await page.getByLabel('Repository 網址',{exact:true}).fill('Owner/Repo');await page.getByLabel('細粒度 PAT',{exact:true}).fill('synthetic-retry');await page.getByRole('button',{name:'確認目的地',exact:true}).click();
 await expect(page.getByRole('alert')).toBeVisible();await page.getByRole('button',{name:'確認目的地',exact:true}).click();await expect(page.getByText('尚未驗證 Issues 寫入權限。',{exact:true})).toBeVisible();expect(credentials).toBe(1);expect(inspections).toBe(2);await page.getByRole('button',{name:'啟用 GitHub 紀錄',exact:true}).click();await expect(page.getByText('GitHub 紀錄已啟用。',{exact:true})).toBeVisible();
});

test('guided GitHub confirms metadata without claiming Issues write permission',async({page,service,activate})=>{
 await activate();await seedProject(page,service);await page.goto(service.origin+'/settings');
 let calls=0;await page.route('**/commands/binding.inspect',async route=>{calls++;await route.fulfill({contentType:'application/json',body:JSON.stringify({result:{repository:'Owner/Repo',repositoryId:42,private:true,hasIssues:true,issuesWriteVerified:false}})})});
 await page.getByRole('button',{name:'連接 GitHub',exact:true}).click();
 await page.getByLabel('Repository 網址',{exact:true}).fill('https://github.com/Owner/Repo');await page.getByLabel('細粒度 PAT',{exact:true}).fill('synthetic-pat');
 await page.getByRole('button',{name:'確認目的地',exact:true}).click();
 await expect(page.getByText('尚未驗證 Issues 寫入權限。',{exact:true})).toBeVisible();expect(calls).toBe(1);
 expect(await ownerRead(page,service,'bindings',undefined,(await ownerRead(page,service,'projects'))[0].id)).toHaveLength(0);
 await page.getByRole('button',{name:'啟用 GitHub 紀錄',exact:true}).click();
 await expect(page.getByText('GitHub 紀錄已啟用。',{exact:true})).toBeVisible();
 const binding=(await ownerRead(page,service,'bindings',undefined,(await ownerRead(page,service,'projects'))[0].id))[0];expect(binding.repositoryId).toBe(42);expect(binding.repository).toBe('Owner/Repo');
 expect(await page.evaluate(()=>JSON.stringify({...localStorage,...sessionStorage}))).not.toContain('synthetic-pat');
});

test('guided DingTalk accepts complete official URL without a test send',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);await page.goto(service.origin+'/settings');
 let commands:string[]=[];page.on('request',r=>{if(r.method()==='POST')commands.push(r.url())});
 await page.getByRole('button',{name:'連接釘釘',exact:true}).click();
 await page.getByLabel('完整 Webhook 網址',{exact:true}).fill('https://oapi.dingtalk.com/robot/send?access_token=synthetic-notification');await page.getByLabel('加簽 secret',{exact:true}).first().fill('synthetic-sign');
 await page.getByRole('button',{name:'確認通知範圍',exact:true}).click();await expect(page.getByText('只通知待拍板、人工驗收與需你處理的阻塞。',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'啟用釘釘通知',exact:true}).click();await expect(page.getByText('釘釘通知已啟用。',{exact:true})).toBeVisible();
 expect(await ownerRead(page,service,'bindings',undefined,project.id)).toHaveLength(1);expect(await ownerRead(page,service,'outbound-jobs',undefined,project.id)).toHaveLength(0);expect(commands.every(url=>url.startsWith(service.origin))).toBe(true);
});
