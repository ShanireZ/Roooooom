import {execFileSync} from 'node:child_process';
import {resolve,posix} from 'node:path';

const root=resolve(process.argv.includes('--root')?process.argv[process.argv.indexOf('--root')+1]:'.');
const index=execFileSync('git',['ls-files','--stage','-z'],{cwd:root,encoding:'utf8'}).split('\0').filter(Boolean).map(record=>{
 const match=record.match(/^([0-7]+) ([a-f0-9]+) (\d)\t([\s\S]+)$/);if(!match||match[3]!=='0')throw Error('Unmerged or invalid index');return {mode:match[1],oid:match[2],path:match[4]};
});
const output=execFileSync('git',['cat-file','--batch'],{cwd:root,input:index.map(x=>x.oid).join('\n')+'\n',maxBuffer:32*1024*1024});
let offset=0;const files=new Map(),errors=[];
for(const entry of index){const line=output.indexOf(10,offset),header=output.subarray(offset,line).toString('utf8').split(' '),size=Number(header[2]);if(header[1]!=='blob'||!Number.isSafeInteger(size))throw Error('Only ordinary staged blobs may be published');const bytes=output.subarray(line+1,line+1+size);offset=line+1+size+1;if(entry.mode!=='100644'&&entry.mode!=='100755')errors.push('non-regular-file: '+entry.path);files.set(entry.path,bytes.includes(0)?null:bytes.toString('utf8'))}
const translations=new Set(['README.zh-CN.md','README.zh-TW.md','README.ja.md']);
const privatePath=/(?:^|\/)(?:\.local|\.roooooom|\.superpowers|\.scratch|node_modules|dist|bin|coverage|credentials|backups|exports|evidence-private|handoff|\.idea|\.vscode)(?:\/|$)|(?:^|\/)(?:AGENTS\.md|PLAN\.md|activation\.html|discovery\.json)$|^docs\/superpowers\/|(?:^|\/)\.env(?!\.example$)(?:\.|$)|\.(?:sqlite3?|db)(?:-wal|-shm)?$|\.(?:pem|key|exe|zip|dump|log)$|\.tar\.gz$/i;
const secret=/\b(?:gh[pousr]_[A-Za-z0-9]{20,255}|github_pat_[A-Za-z0-9_]{30,255}|AKIA[0-9A-Z]{16}|sk-[A-Za-z0-9]{25,255})\b|-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----[\r\n]/;
for(const [name,text] of files){
 if(privatePath.test(name))errors.push('private-path: '+name);
 if(text===null)continue;
 if(secret.test(text))errors.push('secret-like: '+name);
 if(/[A-Z]:[\\/]+Users[\\/]|D:[\\/]+WorkSpace/i.test(text))errors.push('machine-private-reference: '+name);
 if(name.endsWith('.md')&&!translations.has(name)&&/[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}]/u.test(text))errors.push('english-doc: '+name);
 if(name.endsWith('.md'))for(const match of text.matchAll(/!?\[[^\]\n]*\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g)){
  const target=match[1].replace(/^<|>$/g,'');if(/^[a-z][a-z0-9+.-]*:|^#|^\/\//i.test(target))continue;
  const path=posix.normalize(posix.join(posix.dirname(name),decodeURIComponent(target.split('#')[0])));if(!files.has(path))errors.push('local-link: '+name+' -> '+path);
 }
}
for(const name of ['.gitignore','README.md',...translations,'LICENSE','COPYRIGHT'])if(!files.has(name))errors.push('missing-required: '+name);
const license=files.get('LICENSE')??'';if(!license.includes('GNU GENERAL PUBLIC LICENSE')||!license.includes('Version 3, 29 June 2007'))errors.push('gpl-license-text: LICENSE');
for(const name of ['web/package.json','packaging/platforms.json']){try{if(JSON.parse(files.get(name)).license!=='GPL-3.0-only')errors.push('gpl-metadata: '+name)}catch{errors.push('invalid-license-metadata: '+name)}}
if(errors.length){for(const error of errors)console.error(error);process.exitCode=1}else console.log('Publication check passed: '+files.size+' staged files; English technical docs, four README editions, GPL-3.0-only, local links, private-path and recognizable-secret guards. Inspect outgoing history separately; this is not exhaustive secret or legal certification.');
