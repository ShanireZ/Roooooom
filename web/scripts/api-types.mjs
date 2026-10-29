import {readFile,writeFile,mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import openapiTS,{astToString} from 'openapi-typescript';
const source=new URL('../../api/openapi.yaml',import.meta.url);
const destination=new URL('../src/api/contracts.generated.ts',import.meta.url);
const text='// Generated from api/openapi.yaml. Do not edit.\n'+astToString(await openapiTS(source));
if(process.argv.includes('--check')){
 const root=await mkdtemp(join(tmpdir(),'roooooom-api-check-'));
 try {
  const candidate=join(root,'contracts.generated.ts');await writeFile(candidate,text);
  if(await readFile(destination,'utf8')!==await readFile(candidate,'utf8'))throw new Error('API types drift; run npm run api:generate');
 }finally{await rm(root,{recursive:true,force:true})}
}else{await writeFile(destination,text)}
