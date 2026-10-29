export type TaskTab='overview'|'work'|'evidence'|'discussion'|'history'|'manage';
export type SettingsSection='door'|'members'|'integrations';
export type DrawerRoute={kind:'task';id:string;tab:TaskTab}|{kind:'request';id:string}|{kind:'decision';id:string}|{kind:'acceptance';taskId:string}|{kind:'archive'}|{kind:'settings';section:SettingsSection};
export type Route={name:'home'}|{name:'newRoom'}|{name:'room';projectId:string;drawer?:DrawerRoute}|{name:'settings'}
 |{name:'legacyTask';id:string}|{name:'legacyRequest';id:string}|{name:'legacy';to:'home'|'needsYou'|'newRoom'|'roomDoor';projectId?:string}|{name:'notFound'};

const TABS:readonly TaskTab[]=['overview','work','evidence','discussion','history','manage'];
const SECTIONS:readonly SettingsSection[]=['door','members','integrations'];
const id=(value:string|undefined):value is string=>!!value&&/^[A-Za-z0-9_-]{1,128}$/.test(value);
const notFound:Route={name:'notFound'};

// `new` and `settings` after /rooms/ are reserved and matched before a project id.
export function parseRoute(path:string):Route{
 const parts=path.split('/').slice(1);
 if(path==='/')return {name:'home'};
 if(path==='/projects')return {name:'legacy',to:'home'};
 if(path==='/decisions')return {name:'legacy',to:'needsYou'};
 if(path==='/setup'||path==='/setup/new')return {name:'legacy',to:'newRoom'};
 if(parts[0]==='setup'&&parts.length===2&&id(parts[1]))return {name:'legacy',to:'roomDoor',projectId:parts[1]};
 if(parts[0]==='tasks'&&parts.length===2&&id(parts[1]))return {name:'legacyTask',id:parts[1]};
 if(parts[0]==='requests'&&parts.length===2&&id(parts[1]))return {name:'legacyRequest',id:parts[1]};
 if(path==='/settings'||path==='/rooms/settings')return {name:'settings'};
 if(path==='/rooms/new')return {name:'newRoom'};
 if(parts[0]!=='rooms'||!id(parts[1]))return notFound;
 const [, projectId,kind,target,extra]=parts;
 if(parts.length===2)return {name:'room',projectId};
 const room=(drawer:DrawerRoute):Route=>({name:'room',projectId,drawer});
 if(kind==='archive'&&parts.length===3)return room({kind:'archive'});
 if(kind==='settings'&&parts.length<=4){
  const section=(target??'door') as SettingsSection;
  return SECTIONS.includes(section)?room({kind:'settings',section}):notFound;
 }
 if(!id(target))return notFound;
 if(kind==='tasks'&&parts.length<=5){
  const tab=(extra??'overview') as TaskTab;
  return TABS.includes(tab)?room({kind:'task',id:target,tab}):notFound;
 }
 if(parts.length!==4)return notFound;
 if(kind==='requests')return room({kind:'request',id:target});
 if(kind==='decisions')return room({kind:'decision',id:target});
 if(kind==='acceptance')return room({kind:'acceptance',taskId:target});
 return notFound;
}

// Canonical paths omit default segments (the overview tab, the door section).
export function routePath(route:Route):string{
 switch(route.name){
 case 'home':return '/';
 case 'newRoom':return '/rooms/new';
 case 'settings':return '/rooms/settings';
 case 'room':{
  const base='/rooms/'+route.projectId,d=route.drawer;
  if(!d)return base;
  switch(d.kind){
  case 'task':return base+'/tasks/'+d.id+(d.tab==='overview'?'':'/'+d.tab);
  case 'request':return base+'/requests/'+d.id;
  case 'decision':return base+'/decisions/'+d.id;
  case 'acceptance':return base+'/acceptance/'+d.taskId;
  case 'archive':return base+'/archive';
  case 'settings':return base+'/settings'+(d.section==='door'?'':'/'+d.section);
  }
 }
 // eslint-disable-next-line no-fallthrough
 default:return '/';
 }
}
