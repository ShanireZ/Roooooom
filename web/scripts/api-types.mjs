import {readFile,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {renderContractsFile} from './lib/openapi-types.mjs';
// Usage: api-types.mjs [--check] [--spec <openapi file>] [--out <types file>]. Defaults are the repository files.
const options={check:false,spec:fileURLToPath(new URL('../../api/openapi.yaml',import.meta.url)),out:fileURLToPath(new URL('../src/api/contracts.generated.ts',import.meta.url))};
const args=process.argv.slice(2);
for(let i=0;i<args.length;i++){
 if(args[i]==='--check')options.check=true;
 else if((args[i]==='--spec'||args[i]==='--out')&&i+1<args.length)options[args[i].slice(2)]=resolve(args[++i]);
 else throw new Error('Unknown argument '+args[i]+'. Usage: api-types.mjs [--check] [--spec <openapi file>] [--out <types file>]');
}
let spec;
try{spec=JSON.parse(await readFile(options.spec,'utf8'))}
catch(error){throw new Error(options.spec+' must be JSON-compatible YAML (JSON syntax): '+error.message)}
const text=renderContractsFile(spec);
if(options.check){
 // Git may check files out with CRLF on Windows; the generated content itself is always LF.
 if((await readFile(options.out,'utf8')).replaceAll('\r\n','\n')!==text)throw new Error('API types drift; run npm run api:generate');
}else{await writeFile(options.out,text)}
