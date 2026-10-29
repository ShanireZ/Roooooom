import {test,expect} from '../fixture';
import {ownerCommand,seedProject,seedTask} from '../helpers';

test.use({locale:'en-US'});

type Page=import('@playwright/test').Page;
// WCAG contrast of an element's text against the first opaque background behind it.
const contrast=(page:Page,selector:string)=>page.locator(selector).first().evaluate(el=>{
 const parse=(c:string)=>(c.match(/[\d.]+/g)??[]).map(Number);
 let fg=parse(getComputedStyle(el).color),bg=[255,255,255,1];
 for(let n:Element|null=el;n;n=n.parentElement){const b=parse(getComputedStyle(n).backgroundColor);if((b[3]??1)>0){bg=b;break}}
 const a=fg[3]??1;fg=[0,1,2].map(i=>fg[i]*a+bg[i]*(1-a));
 const lum=(c:number[])=>{const v=c.map(x=>{x/=255;return x<=.03928?x/12.92:((x+.055)/1.055)**2.4});return .2126*v[0]+.7152*v[1]+.0722*v[2]};
 const [hi,lo]=[lum(fg),lum(bg.slice(0,3))].sort((x,y)=>y-x);
 return (hi+.05)/(lo+.05);
});

for(const scheme of ['light','dark'] as const){
 test(`owner, danger and secondary text meet WCAG AA in ${scheme} mode`,async({page,service,activate})=>{
  await page.emulateMedia({colorScheme:scheme});
  await activate();
  const p=await seedProject(page,service),task=await seedTask(page,service,p,'integrator','Contrast table');
  await ownerCommand(page,service,'task.create',{projectId:p.id,goal:'Risky change',scope:['fixture'],nonScope:[],authorityRefs:['synthetic://owner'],risk:'high',acceptancePolicy:{mode:'integrator',conditions:['tests pass']},collaboratorLimit:0});
  await page.goto(service.origin+'/rooms/'+p.id);
  await expect(page.locator('.room-tab .count')).toBeVisible();
  for(const selector of ['.needs-button','.room-tab .count','.label','.muted'])expect(await contrast(page,selector),selector).toBeGreaterThanOrEqual(4.5);
  await page.goto(service.origin+'/rooms/'+p.id+'/tasks/'+task.id+'/manage');
  await expect(page.locator('.btn-danger').first()).toBeVisible();
  expect(await contrast(page,'.btn-danger'),'.btn-danger').toBeGreaterThanOrEqual(4.5);
 });
}
