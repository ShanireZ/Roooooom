import {readFile} from 'node:fs/promises';
import {test,expect} from '../fixture';
import {enroll,seedProject} from '../helpers';
import {m} from '../i18n';
import {createTranslator} from '../../src/i18n/translator';

test.use({locale:'en-US'});

const ja=createTranslator('ja').t;
const ownerLanguage=async(page:import('@playwright/test').Page,origin:string)=>(await (await page.request.get(origin+'/api/v1/me')).json()).result.ownerLanguage??'';

test('choosing Japanese writes the owner language, switches the UI and survives a reload in an en-US browser',async({page,service,activate})=>{
 await activate();await page.goto(service.origin+'/settings');
 await expect(page.getByRole('heading',{name:m('system.title'),exact:true})).toBeVisible();
 await page.getByLabel(m('system.languageLabel'),{exact:true}).selectOption('ja');
 await expect(page.locator('html')).toHaveAttribute('lang','ja');
 await expect(page.getByRole('heading',{name:ja('system.title'),exact:true})).toBeVisible();
 expect(await ownerLanguage(page,service.origin)).toBe('ja');
 await page.reload();
 await expect(page.getByRole('heading',{name:ja('system.title'),exact:true})).toBeVisible();
 await expect(page.locator('html')).toHaveAttribute('lang','ja');
});

test('Follow browser clears the owner language',async({page,service,activate})=>{
 await activate();await page.goto(service.origin+'/settings');
 await page.getByLabel(m('system.languageLabel'),{exact:true}).selectOption('zh-TW');
 await expect(page.locator('html')).toHaveAttribute('lang','zh-TW');
 await page.locator('select').first().selectOption('');
 await expect(page.locator('html')).toHaveAttribute('lang','en');
 expect(await ownerLanguage(page,service.origin)).toBe('');
 await page.reload();
 await expect(page.getByRole('heading',{name:m('system.title'),exact:true})).toBeVisible();
});

test('the language control is disabled while read-only',async({page,service,activate})=>{
 await activate();await page.goto(service.origin+'/settings');
 await expect(page.getByLabel(m('system.languageLabel'),{exact:true})).toBeEnabled();
 await service.stop();
 await expect(page.getByLabel(m('system.languageLabel'),{exact:true})).toBeDisabled({timeout:15000});
 await expect(page.getByLabel(m('system.theme'),{exact:true})).toBeEnabled();
});

test('theme Dark sets data-theme and survives reload',async({page,service,activate})=>{
 await activate();await page.goto(service.origin+'/settings');
 await page.getByLabel(m('system.theme'),{exact:true}).selectOption('dark');
 await expect(page.locator('html')).toHaveAttribute('data-theme','dark');
 await page.reload();
 await expect(page.locator('html')).toHaveAttribute('data-theme','dark');
 await expect(page.getByLabel(m('system.theme'),{exact:true})).toHaveValue('dark');
});

test('backup shows the private path and exports without credentials; storage shows the warning style',async({page,service,activate})=>{
 await activate();const project=await seedProject(page,service);const agent=await enroll(page,service,project.id,'Backup agent');
 await page.route('**/api/v1/queries/metrics',async route=>{await route.fulfill({contentType:'application/json',body:JSON.stringify({result:{storage:{databaseBytes:805306368,hardBytes:1073741824,warning:true},counters:{commands:12,rejected:1,commandP95Millis:3.5}}})})});
 await page.goto(service.origin+'/settings');
 await expect(page.getByTestId('storage')).toHaveClass(/warning/);
 await expect(page.getByTestId('storage')).toContainText(m('system.storageWarning'));
 const snapshotResponse=page.waitForResponse(r=>r.url().endsWith('/commands/backup.create'));
 await page.getByRole('button',{name:m('system.snapshot'),exact:true}).click();
 const snapshot=(await (await snapshotResponse).json()).result;
 await expect(page.getByTestId('private-path')).toContainText(snapshot.path);
 await expect(page.getByTestId('private-path')).toContainText(m('system.privatePath'));
 const manifest=JSON.parse(await readFile(snapshot.path+'/manifest.json','utf8'));expect(manifest.sha256).toHaveLength(64);
 const exportResponse=page.waitForResponse(r=>r.url().endsWith('/commands/backup.export'));
 await page.getByRole('button',{name:m('system.export'),exact:true}).click();
 const exported=(await (await exportResponse).json()).result;
 await expect(page.getByTestId('private-path')).toContainText(exported.path);
 const data=await readFile(exported.path+'/data.json','utf8');
 expect(data).not.toContain(agent.token);expect(data).not.toContain('"tokens"');
});

test('About shows the build fingerprint',async({page,service,activate})=>{
 await activate();
 await page.route('**/api/v1/me',async route=>{const response=await route.fetch();const body=await response.json();body.result.build={...body.result.build,metadata:{built:true,version:'0.1.0-dev',commit:'0123456789abcdef',dirty:false,sourceFingerprint:'f'.repeat(64)}};await route.fulfill({response,json:body})});
 await page.goto(service.origin+'/settings');
 const about=page.getByRole('region',{name:m('system.about')});
 await expect(about).toContainText('f'.repeat(64));
 await expect(about).toContainText('0.1.0-dev');
 await expect(about).toContainText('0123456789abcdef');
});
