import {test,expect} from './fixture';
import {ownerRead} from './helpers';
import {join} from 'node:path';
import {pathToFileURL} from 'node:url';

test('guided project connection needs no technical handles or integrations',async({page,service})=>{
 await page.goto(pathToFileURL(join(service.root,'activation.html')).href);
 await page.locator('form button[type=submit]').click();
 await page.getByLabel('Repository 路徑或網址',{exact:true}).fill('C:/Projects/SimpleProject');
 await expect(page.getByLabel('專案名稱',{exact:true})).toHaveValue('SimpleProject');
 await expect(page.getByText(/確認接入後.*允許/)).toBeVisible();
 await page.getByRole('button',{name:'確認接入',exact:true}).click();
 await expect(page.getByRole('heading',{name:'把接入說明交給 agent',exact:true})).toBeVisible();
 await expect(page.getByTestId('join-instructions')).toContainText('SimpleProject');
 await expect(page.getByTestId('join-instructions')).toContainText('SKILL.md');
 await expect(page.locator('input[type=password]')).toHaveCount(0);
 const projects=await ownerRead(page,service,'projects');expect(projects).toHaveLength(1);expect(projects[0].enrollmentEnabled).toBe(true);await expect(page.getByTestId('join-instructions')).toContainText('--project '+projects[0].id);
 await page.reload();await expect(page.getByRole('heading',{name:'把接入說明交給 agent',exact:true})).toBeVisible();
 expect(await ownerRead(page,service,'projects')).toHaveLength(1);
});

test('enrollment failure resumes the registered project after refresh',async({page,service,activate})=>{
 await activate();let fail=true;
 await page.route('**/api/v1/commands/enrollment.enable',async route=>{if(fail){fail=false;await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{code:'temporary-unavailable'}})})}else await route.continue()});
 await page.getByLabel('Repository 路徑或網址',{exact:true}).fill('synthetic://ResumeProject');
 await page.getByRole('button',{name:'確認接入',exact:true}).click();
 await expect(page.getByText('專案已登記，尚未開放 agent 接入。',{exact:true})).toBeVisible();
 expect(await ownerRead(page,service,'projects')).toHaveLength(1);
 await page.reload();
 await page.getByRole('button',{name:'開放此專案接入',exact:true}).click();
 await expect(page.getByRole('heading',{name:'把接入說明交給 agent',exact:true})).toBeVisible();
 expect(await ownerRead(page,service,'projects')).toHaveLength(1);
});

test('lost project-create response retries the original request without duplicate project',async({page,service,activate})=>{
 await activate();let lose=true;const ids:string[]=[];
 await page.route('**/api/v1/commands/project.create',async route=>{ids.push(route.request().postDataJSON().clientRequestId);if(lose){lose=false;await route.fetch();await route.abort('failed')}else await route.continue()});
 await page.getByLabel('Repository 路徑或網址',{exact:true}).fill('synthetic://NoDuplicates');
 await page.getByRole('button',{name:'確認接入',exact:true}).click();
 await page.getByRole('button',{name:'重試接入',exact:true}).click();
 await expect(page.getByRole('heading',{name:'把接入說明交給 agent',exact:true})).toBeVisible();
 expect(ids).toHaveLength(2);expect(ids[0]).toBe(ids[1]);expect(await ownerRead(page,service,'projects')).toHaveLength(1);
});
