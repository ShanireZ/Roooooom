import {test,expect} from '../fixture';
import {askDecision,claimIntegrator,enroll,ownerCommand,seedProject,seedTask} from '../helpers';
import {LOCALES} from '../../src/i18n/locale';
import {createTranslator} from '../../src/i18n/translator';

test.use({viewport:{width:375,height:812}});

// Each surface waits for a data-dependent anchor, then checks keys, language and width.
for(const locale of LOCALES){
 test(`every surface renders in ${locale} without missing keys or horizontal scroll`,async({page,service,activate})=>{
  const t=createTranslator(locale).t;
  await activate();
  const project=await seedProject(page,service),task=await seedTask(page,service,project,'integrator','Locale table');
  const agent=await enroll(page,service,project.id,'Locale agent');await claimIntegrator(page,service,agent,task.id);
  const decision=await askDecision(page,service,agent,task.id);
  const request=await ownerCommand(page,service,'request.create',{projectId:project.id,text:'Locale request'});
  await ownerCommand(page,service,'owner.language',{language:locale});
  const room=service.origin+'/rooms/'+project.id;
  const surfaces:[string,string][]=[
   [room,'Locale table'],
   ...['overview','work','evidence','discussion','history','manage'].map(tab=>[room+'/tasks/'+task.id+'/'+tab,'Locale table'] as [string,string]),
   [room+'/requests/'+request.id,'Locale request'],
   [room+'/decisions/'+decision.id,decision.question],
   [room+'/acceptance/'+task.id,t('acceptance.none')],
   [room+'/archive',t('archive.title')],
   [room+'/settings','synthetic://fixture'],
   [room+'/settings/members',t('session.status.active')],
   [room+'/settings/integrations',t('guided.github')],
   [service.origin+'/rooms/new',t('setup.title')],
   [service.origin+'/rooms/settings',t('system.title')],
  ];
  for(const [url,anchor] of surfaces){
   await page.goto(url);
   await expect(page.getByText(anchor).filter({visible:true}).first(),url).toBeVisible();
   await expect(page.locator('html'),url).toHaveAttribute('lang',locale);
   expect(await page.content(),url).not.toContain('[missing:');
   const widths=await page.evaluate(()=>({page:document.documentElement.scrollWidth<=window.innerWidth,dialogs:[...document.querySelectorAll('dialog[open]')].every(d=>d.scrollWidth<=d.clientWidth+1)}));
   expect(widths,url).toEqual({page:true,dialogs:true});
  }
 });
}
