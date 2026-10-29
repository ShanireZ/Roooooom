import {join} from 'node:path';
import {rename,writeFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {ROOT,args,platform,run,webProof,productInputs,hash,writeJSON,safeDirectory,coreName,launcherName} from './lib/packaging.mjs';
export async function buildCore(target){
 const p=await platform(target),web=await webProof(),inputs=await productInputs();
 const version=(await run('go',['version'],{capture:true})).trim();if(!version.includes('go1.27.3 '))throw Error('Go 1.27.3 required');
 await run('go',['mod','verify']);
 const commit=(await run('git',['rev-parse','HEAD'],{capture:true})).trim(),dirty=!!(await run('git',['status','--porcelain'],{capture:true})).trim();
 const metadata={built:true,version:'0.1.0-dev',protocolVersion:1,schemaVersion:2,commit,dirty,sourceFingerprint:hash(JSON.stringify(inputs)),apiHash:inputs['api/openapi.yaml'],webHash:hash(JSON.stringify(web.files))};
 const encoded=Buffer.from(JSON.stringify(metadata)).toString('base64url'),out=await safeDirectory(join(ROOT,'.local/build',target));
 const [goos,goarch]=target.split('-'),env={GOOS:goos,GOARCH:goarch,CGO_ENABLED:'0'};
 for(const [pkg,name,gui] of [['./cmd/roooooom',coreName(target),false],['./cmd/roooooom-launcher',launcherName(target),goos==='windows']]){
  const destination=join(out,name),temporary=destination+'.new';
  const flags=(gui?'-H=windowsgui ':'')+'-X roooooom.local/roooooom/internal/buildinfo.Encoded='+encoded;
  await run('go',['build','-trimpath','-buildvcs=false','-ldflags',flags,'-o',temporary,pkg],{env});
  await rename(temporary,destination);
 }
 await writeJSON(join(out,'build.json'),{...metadata,target,cgoEnabled:false,goVersion:'go1.27.3',nodeVersion:process.version,platform:p,uiFiles:web.files,sourceHashes:inputs});
 console.log('Built standalone '+target+' (platform support remains '+p.supported+').');return out;
}
if(process.argv[1]&&fileURLToPath(import.meta.url)===process.argv[1]){const a=args();if(!a['--target'])throw Error('--target required');await buildCore(a['--target'])}
