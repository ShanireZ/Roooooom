// Fails when product source contains CJK text outside the locale catalogs.
// Unit tests may hold deliberate multilingual fixtures.
import {readdir,readFile} from 'node:fs/promises';
import {join,relative} from 'node:path';
import {fileURLToPath} from 'node:url';
const root=fileURLToPath(new URL('../src/',import.meta.url));
const exempt=['i18n/catalog/'];
const cjk=/[\u3040-\u30ff\u3400-\u9fff\uf900-\ufaff]/;
async function* files(dir){
 for(const entry of await readdir(dir,{withFileTypes:true})){
  const path=join(dir,entry.name);
  if(entry.isDirectory())yield* files(path);
  else if(/\.(ts|tsx)$/.test(entry.name)&&!entry.name.endsWith('.test.ts'))yield path;
 }
}
const problems=[];
for await(const path of files(root)){
 const name=relative(root,path).replaceAll('\\','/');
 if(exempt.some(prefix=>name.startsWith(prefix)))continue;
 (await readFile(path,'utf8')).split('\n').forEach((line,i)=>{if(cjk.test(line))problems.push('src/'+name+':'+(i+1))});
}
if(problems.length){console.error('UI text must come from web/src/i18n/catalog:\n'+problems.join('\n'));process.exit(1)}
console.log('i18n check: no literal CJK text outside catalogs');
