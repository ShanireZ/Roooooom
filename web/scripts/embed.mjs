import {cp,mkdir,rm,readFile,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
const source=new URL('../dist/',import.meta.url);
const destination=new URL('../../internal/webui/dist/',import.meta.url);
const index=await readFile(new URL('index.html',source),'utf8');
if(!index.includes('/assets/')||index.includes('/src/'))throw new Error('Production UI build missing');
// A fixed product build directory only; never accept a caller-selected path.
await rm(destination,{recursive:true,force:true});
await mkdir(destination,{recursive:true});
await cp(source,destination,{recursive:true});
await writeFile(new URL('build.json',destination),JSON.stringify({format:1,indexSha256:createHash('sha256').update(index).digest('hex')})+'\n');
