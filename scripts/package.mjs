import {readFile,writeFile,mkdir,copyFile,chmod,readdir,lstat} from 'node:fs/promises';
import {join,dirname,resolve} from 'node:path';
import {buildWeb} from './build-web.mjs';
import {buildCore} from './build-core.mjs';
import {ROOT,args,platform,run,files,inventory,hash,writeJSON,safeDirectory,coreName,launcherName,json} from './lib/packaging.mjs';
import {zip} from './lib/zip.mjs';
function streamJSON(text){const values=[];let start=-1,depth=0,quoted=false,escaped=false;for(let i=0;i<text.length;i++){const c=text[i];if(quoted){if(escaped)escaped=false;else if(c==='\\')escaped=true;else if(c==='"')quoted=false;continue}if(c==='"'){quoted=true;continue}if(c==='{'){if(depth++===0)start=i}else if(c==='}'&&--depth===0){values.push(JSON.parse(text.slice(start,i+1)));start=-1}}if(depth||quoted)throw Error('Invalid module inventory');return values}
async function notices(bundle){
 const modules=streamJSON(await run('go',['list','-m','-json','all'],{capture:true})),records=[];
 for(const m of modules){if(m.Main)continue;const record={ecosystem:'go',name:m.Path,version:m.Version,sum:m.Sum??'',licenseFiles:[]};if(m.Dir){for(const entry of await readdir(m.Dir)){if(!/^(?:LICENSE(?:\..+)?|COPYING)$/i.test(entry))continue;const path=join(m.Dir,entry);if(!(await lstat(path)).isFile())continue;const dest='LICENSES/go/'+hash(m.Path+'@'+m.Version).slice(0,16)+'/'+entry;await mkdir(dirname(join(bundle,dest)),{recursive:true});await copyFile(path,join(bundle,dest));record.licenseFiles.push(dest)}}record.licenseVerified=record.licenseFiles.length>0;records.push(record)}
 const lock=await json(join(ROOT,'web/package-lock.json'));
 for(const [path,p] of Object.entries(lock.packages)){if(!path||p.dev||!p.version)continue;const name=path.slice(path.lastIndexOf('node_modules/')+13),record={ecosystem:'npm',name,version:p.version,integrity:p.integrity??'',declaredLicense:p.license??'unverified',licenseFiles:[]};const dir=join(ROOT,'web',path);for(const entry of await readdir(dir)){if(!/^LICENSE(?:\..+)?$/i.test(entry))continue;const source=join(dir,entry);if(!(await lstat(source)).isFile())continue;const dest='LICENSES/npm/'+hash(name+'@'+p.version).slice(0,16)+'/'+entry;await mkdir(dirname(join(bundle,dest)),{recursive:true});await copyFile(source,join(bundle,dest));record.licenseFiles.push(dest)}record.licenseVerified=record.licenseFiles.length>0;records.push(record)}
 const goroot=(await run('go',['env','GOROOT'],{capture:true})).trim();await mkdir(join(bundle,'LICENSES/go-toolchain'),{recursive:true});await copyFile(join(goroot,'LICENSE'),join(bundle,'LICENSES/go-toolchain/LICENSE'));
 records.sort((a,b)=>(a.ecosystem+a.name).localeCompare(b.ecosystem+b.name,'en'));
 await writeJSON(join(bundle,'dependencies.json'),{format:1,records});
 await writeFile(join(bundle,'THIRD_PARTY_NOTICES.md'),'# Third-party notices\n\nPinned dependencies and discovered upstream license texts are included in dependencies.json and LICENSES/. Missing texts stay unverified. This is not a project distribution license or a legal compliance certification. Project-owned material is GPL-3.0-only; dependencies retain their own terms. Binary release certification is separate from source publication.\n');
}
async function copyText(source,dest){await mkdir(dirname(dest),{recursive:true});await writeFile(dest,(await readFile(source,'utf8')).replaceAll('\r\n','\n'))}
export async function packageBundle(target,out,{skipWeb=false}={}){
 const selected=await platform(target),parent=await safeDirectory(resolve(out)),bundle=join(parent,'roooooom-'+target);
 await mkdir(bundle,{mode:0o700}); // New-only: never erase or replace another output.
 if(!skipWeb)await buildWeb();
 const built=await buildCore(target),info=await json(join(built,'build.json'));
 for(const name of [coreName(target),launcherName(target)]){await copyFile(join(built,name),join(bundle,name));await chmod(join(bundle,name),0o755)}
 await copyText(join(ROOT,'packaging/README.md'),join(bundle,'README.md'));
 await copyText(join(ROOT,'packaging',target.startsWith('darwin-')?'macos':target.split('-')[0],'README.md'),join(bundle,'PLATFORM.md'));
 await copyText(join(ROOT,'packaging/platforms.json'),join(bundle,'PLATFORMS.json'));
 for(const path of await files(join(ROOT,'skills/roooooom')))await copyText(join(ROOT,'skills/roooooom',path),join(bundle,'skills/roooooom',path));
 for(const path of ['LICENSE','COPYRIGHT','api/openapi.yaml','go.mod','go.sum','web/package-lock.json'])await copyText(join(ROOT,path),join(bundle,path));
 await notices(bundle);
 const payload=await inventory(bundle);await writeFile(join(bundle,'SHA256SUMS'),Object.entries(payload).map(([p,h])=>h+'  '+p+'\n').join(''));
 const hashes=await inventory(bundle);
 const manifest={format:1,product:'Roooooom',version:info.version,target,commit:info.commit,dirty:info.dirty,sourceFingerprint:info.sourceFingerprint,sourceHashes:info.sourceHashes,protocolVersion:1,schemaVersion:info.schemaVersion,cgoEnabled:false,goVersion:info.goVersion,nodeVersion:info.nodeVersion,npmVersion:'12.2.1',apiHash:info.apiHash,webHash:info.webHash,uiFiles:info.uiFiles,supported:selected.supported,platform:selected,distribution:'development-candidate',signing:selected.signing,license:'GPL-3.0-only',files:hashes};
 await writeJSON(join(bundle,'manifest.json'),manifest);
 const archive=bundle+'.zip';await zip(bundle,archive,[coreName(target),launcherName(target)]);await writeFile(archive+'.sha256',hash(await readFile(archive))+'  '+archive.slice(dirname(archive).length+1)+'\n',{flag:'wx'});
 console.log('Local bundle: '+bundle);console.log('Archive: '+archive);return bundle;
}
const a=args();if(!a['--target']||typeof a['--out']!=='string')throw Error('--target and --out required');await packageBundle(a['--target'],a['--out'],{skipWeb:!!a['--skip-web']});
