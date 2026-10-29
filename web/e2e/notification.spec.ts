import {test,expect} from './fixture';
import {seedProject,ownerRead} from './helpers';
test('owner stores masked integration credentials and explicitly enables a notification binding',async({page,service,activate},testInfo)=>{
 await activate();await seedProject(page,service);await page.getByRole('link',{name:'設定與恢復',exact:true}).click();
 await page.getByText('進階綁定與憑證管理',{exact:true}).click();
 await page.getByLabel('憑證類型',{exact:true}).selectOption('dingtalk',{timeout:5000});
 await page.getByLabel('Token（只存入本產品私有資料）',{exact:true}).fill('synthetic-webhook-token');
 await page.getByLabel('加簽 secret',{exact:true}).fill('SECsynthetic-test');
 const saved=page.waitForResponse(r=>r.url().endsWith('/commands/binding.credential'));
 await page.getByRole('button',{name:'儲存私有憑證',exact:true}).click();
 const response=await (await saved).json();expect(JSON.stringify(response)).not.toContain('synthetic-webhook-token');expect(JSON.stringify(response)).not.toContain('SECsynthetic-test');
 await expect(page.getByLabel('Token（只存入本產品私有資料）',{exact:true})).toHaveValue('');
 await expect(page.getByLabel('釘釘私有憑證參考',{exact:true})).toHaveValue(response.result.credentialRef);
 await page.getByText('通知格式預覽',{exact:true}).click();
 await expect(page.getByText('手機只能收提醒，請回這台電腦的決策頁正式回覆。',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'建立釘釘通知綁定',exact:true}).click();
 await expect(page.getByText('釘釘通知綁定已建立。',{exact:true})).toBeVisible();
 const bindings=await ownerRead(page,service,'bindings');expect(bindings).toHaveLength(1);expect(bindings[0].kind).toBe('dingtalk');
 expect(JSON.stringify(bindings)).not.toContain('synthetic-webhook-token');
 // No task or pending decision is created: enabling is not a test send.
 expect(await ownerRead(page,service,'outbound-jobs')).toHaveLength(0);
 await page.setViewportSize({width:1440,height:960});await page.screenshot({path:testInfo.outputPath('notification-settings-desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:testInfo.outputPath('notification-settings-mobile.png'),fullPage:true});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
});
