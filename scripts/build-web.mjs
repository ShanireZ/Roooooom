import {mkdir} from 'node:fs/promises';
import {join} from 'node:path';
import {ROOT,args,run,npmCLI,webInputs,inventory,hash,writeJSON} from './lib/packaging.mjs';
export async function buildWeb(install=true){
 if(process.version!=='v24.22.1')throw Error('Build requires Node v24.22.1');
 if(install){const npm=await npmCLI();await run(process.execPath,[npm,'--prefix',join(ROOT,'web'),'ci'])}
 await run(process.execPath,[join(ROOT,'web/scripts/api-types.mjs'),'--check']);
 await run(process.execPath,[join(ROOT,'web/node_modules/typescript/bin/tsc'),'--noEmit'],{cwd:join(ROOT,'web')});
 await run(process.execPath,[join(ROOT,'web/node_modules/vitest/vitest.mjs'),'run'],{cwd:join(ROOT,'web')});
 await run(process.execPath,[join(ROOT,'web/node_modules/vite/bin/vite.js'),'build'],{cwd:join(ROOT,'web')});
 await run(process.execPath,[join(ROOT,'web/scripts/embed.mjs')]);
 const inputs=await webInputs();await mkdir(join(ROOT,'.local/build'),{recursive:true});
 await writeJSON(join(ROOT,'.local/build/web-proof.json'),{format:1,sourceFingerprint:hash(JSON.stringify(inputs)),sourceHashes:inputs,files:await inventory(join(ROOT,'internal/webui/dist')),nodeVersion:process.version,npmVersion:'12.2.1'});
 console.log('Fresh embedded Web proof recorded.');
}
if(process.argv[1]&&new URL('file:'+process.argv[1].replaceAll('\\','/')).pathname.endsWith('/build-web.mjs')){const a=args();await buildWeb(!a['--no-install'])}
