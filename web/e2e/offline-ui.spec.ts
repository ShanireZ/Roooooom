import {test,expect} from './fixture';
import {seedProject,seedTask} from './helpers';
test('UnsafeMarkdownAndDisconnectedUI: outage keeps the snapshot read-only',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);await seedTask(page,service,project,'integrator','離線時保留的任務');
 await page.getByRole('link',{name:'專案',exact:true}).click();
 await expect(page.getByText('離線時保留的任務',{exact:true})).toBeVisible();
 await service.stop();
 await expect(page.getByText('已離線 · 僅供檢視',{exact:true})).toBeVisible();
 await expect(page.getByText('離線時保留的任務',{exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:'新增需求',exact:true})).toBeDisabled();
 expect(await page.evaluate(()=>Object.keys(localStorage))).toEqual([]);
});
