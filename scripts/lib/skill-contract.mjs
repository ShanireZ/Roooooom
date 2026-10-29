import {createHash} from 'node:crypto';

const begin='<!-- ROOOOOOM:BEGIN',end='<!-- ROOOOOOM:END -->';
const digest=text=>createHash('sha256').update(text.replaceAll('\r\n','\n'),'utf8').digest('hex');
const secretPattern=/ROOOOOOM_TEST_SECRET_SENTINEL|github_pat_[A-Za-z0-9_]+|ghp_[A-Za-z0-9]+|Bearer\s+[A-Za-z0-9_-]{30,}|["'](?:token|capability|csrf|pat)["']\s*:\s*["'][A-Za-z0-9_-]{20,}/i;
const keys=['protocolVersion','projectId','serviceUrlRef','profileRef','skillRef','ownerApprovalRef'];
function render(template,context,eol){
 if(context.protocolVersion!==1)return;
 for(const key of keys){
  const value=context[key];
  if(key==='protocolVersion')continue;
  if(typeof value!=='string'||!value.trim()||value.length>512||/[\r\n\0<>]/.test(value)||secretPattern.test(value))return;
 }
 if(!/^[A-Za-z0-9_-]{1,128}$/.test(context.projectId))return;
 let body=template.replaceAll('\r\n','\n').trimEnd();
 for(const key of keys)body=body.replaceAll('{{'+key+'}}',key==='protocolVersion'?'1':JSON.stringify(context[key]));
 if(/\{\{[^}]+\}\}/.test(body))return;
 return (('\n\n'+begin+' v1 sha256='+digest(body)+' -->\n'+body+'\n'+end+'\n')).replaceAll('\n',eol);
}
// Pure proposals: neither this helper nor the product opens or writes AGENTS.md.
export function editGuide(input,template,context,operation='add'){
 const review=()=>({result:'needs-review',text:input});
 if(!['add','remove'].includes(operation))return review();
 const starts=[...input.matchAll(/<!-- ROOOOOOM:BEGIN/g)],ends=[...input.matchAll(/<!-- ROOOOOOM:END -->/g)];
 const desired=render(template,context,input.includes('\r\n')?'\r\n':'\n');
 if(!desired)return review();
 if(!starts.length&&!ends.length)return operation==='remove'?{result:'unchanged',text:input}:{result:'proposed',text:input+desired};
 if(starts.length!==1||ends.length!==1)return review();
 const start=starts[0].index,finish=ends[0].index;
 if(finish<start)return review();
 const header=input.slice(start).match(/^<!-- ROOOOOOM:BEGIN v1 sha256=([a-f0-9]{64}) -->(\r?\n)/);
 if(!header)return review();
 const eol=header[2],prefix=eol+eol,ownedStart=start-prefix.length,ownedEnd=finish+end.length+eol.length;
 if(ownedStart<0||input.slice(ownedStart,start)!==prefix||input.slice(finish+end.length,ownedEnd)!==eol)return review();
 const body=input.slice(start+header[0].length,finish-eol.length);
 if(digest(body)!==header[1])return review();
 const actual=input.slice(ownedStart,ownedEnd),expected=render(template,context,eol);
 if(actual!==expected)return review();
 if(operation==='add')return {result:'unchanged',text:input};
 return {result:'proposed',text:input.slice(0,ownedStart)+input.slice(ownedEnd)};
}

const flags=new Set(['--json','--profile','--data-dir','--project','--session','--target','--cursor','--limit','--seconds','--renew-budget','--credential-file']);
const ownerCommands=/^(project|enrollment|participant|binding|backup)\.|^decision\.(resolve|supersede|withdraw)$|^request\.(create|assign|cancel)$|^service\./;
function commandKind(words){
 const [family,action]=words;
 if(family==='apply')return action;
 if(family==='claim')return 'assignment.claim';
 if(family==='deliver')return 'delivery.'+action;
 if(family==='chat')return action==='send'?'message.send':action==='receipt'?'receipt.update':null;
 if(family==='offline')return action==='resolve'?'offline.resolve':null;
 if(['task','request'].includes(family)&&['get','list'].includes(action))return null;
 if(['task','request','assignment','integrator','session','decision','integration','acceptance','collaborator'].includes(family))return family+'.'+action;
 return null;
}
export function validateInstructions(documents,help){
 const failures=[];
 if(help.protocolVersion!==1)failures.push('protocol version mismatch');
 const intake=documents.join('\n').match(/<!-- ROOOOOOM:INTAKE-CONTRACT -->([\s\S]*?)<!-- ROOOOOOM:INTAKE-CONTRACT-END -->/);
 if(!intake||!['preparation: explicit-claim','task-contract: complete-no-placeholders','source-requirements: immutable','decision-subject: exactly-one'].every(rule=>intake[1].split(/\r?\n/).map(x=>x.trim()).includes(rule)))failures.push('intake preparation or source contract missing');
 for(const document of documents){
  if(secretPattern.test(document))failures.push('secret material in skill');
  for(const match of document.matchAll(/\broooooom[ \t]+([^\n`]+)/g)){
   const words=match[1].trim().split(/\s+/);
   const family=words[0],kind=commandKind(words);
   if(family!=='help'&&!help.cli.includes(family)){failures.push('unknown command family: '+family);continue}
   for(const word of words.filter(x=>x.startsWith('--')))if(!flags.has(word))failures.push('unknown or secret command flag: '+word);
   if(kind&&(!Object.hasOwn(help.commands,kind)||ownerCommands.test(kind)))failures.push('unknown or owner-only command: '+kind);
   if(family==='read'&&!help.queries.includes(words[1]))failures.push('unknown query command: '+words[1]);
   if(family==='help'&&words[1]==='--json'&&words[2]&&!Object.hasOwn(help.commands,words[2]))failures.push('unknown help command: '+words[2]);
   if(family==='offline'&&!['list','reconcile','resolve'].includes(words[1]))failures.push('unknown offline command');
  }
 }
 return failures;
}
