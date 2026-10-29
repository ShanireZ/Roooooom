import {test,expect} from './fixture';
import {seedProject,ownerRead} from './helpers';
test('owner explicitly configures and disables a pinned outbound binding',async({page,service,activate})=>{
 await activate();await seedProject(page,service);
 await page.getByRole('link',{name:'設定與恢復',exact:true}).click();
 await page.getByText('進階綁定與憑證管理',{exact:true}).click();
 await page.getByLabel('GitHub repository（owner/repo）',{exact:true}).fill('synthetic/records',{timeout:5000});
 await page.getByLabel('Canonical repository ID',{exact:true}).fill('42');
 await page.getByLabel('PAT 私有檔參考',{exact:true}).fill('synthetic-test-only');
 await page.getByRole('button',{name:'建立 GitHub 綁定',exact:true}).click();
 await expect(page.getByText('GitHub 綁定已建立。',{exact:true})).toBeVisible();
 expect((await ownerRead(page,service,'bindings'))[0].enabled).toBe(true);
 await page.getByRole('button',{name:'停用綁定',exact:true}).click();
 await expect(page.getByText('已停用；舊工作不會轉投新綁定。',{exact:true})).toBeVisible();
 expect((await ownerRead(page,service,'bindings'))[0].enabled).toBe(false);
});
