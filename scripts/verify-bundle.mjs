import {readFile,writeFile,copyFile,mkdir,mkdtemp,rm,lstat} from 'node:fs/promises';
import {join,resolve,dirname,basename} from 'node:path';
import {tmpdir} from 'node:os';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
import {args,files,hash,json,inventory,writeJSON,nativeTarget,coreName,launcherName,run,ROOT,webProof} from './lib/packaging.mjs';
const entry=fileURLToPath(import.meta.url);
const sha=/^[0-9a-f]{64}$/;
function relativeFile(p){return typeof p==='string'&&p&&!/^(?:[A-Za-z]:|\/|\\)/.test(p)&&!p.includes('\\')&&!p.split('/').some(x=>x==='.'||x==='..'||x==='')}
function environment(profile){const e={PATH:process.platform==='win32'?join(process.env.SystemRoot,'System32'):'/usr/bin:/bin',LOCALAPPDATA:profile,APPDATA:profile};for(const k of ['SystemRoot','WINDIR','TEMP','TMP'])if(process.env[k])e[k]=process.env[k];return e}
async function cleanup(path,prefix){const p=resolve(path);if(dirname(p)!==resolve(tmpdir())||!basename(p).startsWith(prefix))throw Error('Refusing non-owned temp cleanup');await rm(p,{recursive:true,force:true})}
async function binaryJSON(path,argv,env){return new Promise((resolve,reject)=>{const p=spawn(path,argv,{env,windowsHide:true,stdio:['ignore','pipe','pipe']});let output='';const timer=setTimeout(()=>{p.kill();reject(Error('Native metadata probe timed out'))},5000);p.stdout.on('data',b=>{output+=b;if(output.length>1024*1024)p.kill()});p.once('error',e=>{clearTimeout(timer);reject(e)});p.once('exit',code=>{clearTimeout(timer);try{if(code!==0)throw Error('Native metadata probe failed');resolve(JSON.parse(output))}catch(e){reject(e)}})})}
function loopback(address){const match=/^127\.0\.0\.1:(\d{1,5})$/.exec(address??'');return match&&Number(match[1])>0&&Number(match[1])<=65535}
async function requiredResponse(url){const response=await fetch(url,{redirect:'error',signal:AbortSignal.timeout(2000)});if(response.status!==200)throw Error('embedded UI missing: '+new URL(url).pathname);return response}
async function nativeSmoke(bundle,m){
 const root=await mkdtemp(join(tmpdir(),'roooooom-bundle-private-')),env=environment(root);
 let child,exited,resolveExit;exited=new Promise(r=>resolveExit=r);
 try{
  for(const [name,argv] of [[coreName(m.target),['version','--json']],[launcherName(m.target),['--version']]]){
   const v=await binaryJSON(join(bundle,name),argv,env);
   if(v.metadata?.built!==true||v.metadata.sourceFingerprint!==m.sourceFingerprint||v.metadata.commit!==m.commit||v.metadata.apiHash!==m.apiHash||v.metadata.webHash!==m.webHash||v.goVersion!==m.goVersion||v.cgoEnabled!==false||v.goos+'-'+v.goarch!==m.target)throw Error('Native binary metadata differs from manifest');
  }
  child=spawn(join(bundle,coreName(m.target)),['serve','--data-dir',root,'--listen','127.0.0.1:0'],{env,windowsHide:true,stdio:'ignore'});child.once('exit',resolveExit);child.once('error',resolveExit);
  let discovery;const deadline=Date.now()+10000;
  while(Date.now()<deadline){try{const d=await json(join(root,'discovery.json'));if(!loopback(d.listenAddr))throw Error('non-loopback artifact discovery');const r=await fetch('http://'+d.listenAddr+'/identity',{redirect:'error',signal:AbortSignal.timeout(500)});const i=await r.json();if(r.ok&&i.status==='ok'&&i.protocolVersion===1&&i.serviceId===d.serviceId&&i.datasetGeneration===d.datasetGeneration){discovery=d;break}}catch{}if(child.exitCode!==null)break;await new Promise(r=>setTimeout(r,25))}
  if(!discovery)throw Error('Native artifact did not publish a ready own service');
  for(const [path,expected] of Object.entries(m.uiFiles)){if(path!=='index.html'&&!path.startsWith('assets/'))continue;const response=await requiredResponse('http://'+discovery.listenAddr+(path==='index.html'?'/':'/'+path));const bytes=Buffer.from(await response.arrayBuffer());if(bytes.length>4*1024*1024||hash(bytes)!==expected)throw Error('embedded UI differs: '+path)}
  // Only this private test instance is stopped. No supplied PID/process or
  // third-party service is ever a cleanup target.
  try{const c=await json(join(root,'control.json'));if(c.listenAddr===discovery.listenAddr&&c.serviceId===discovery.serviceId&&c.datasetGeneration===discovery.datasetGeneration)await fetch('http://'+c.listenAddr+'/api/v1/control/stop',{method:'POST',headers:{Authorization:'Bearer '+c.token,'Content-Type':'application/json'},body:JSON.stringify({serviceId:c.serviceId,datasetGeneration:c.datasetGeneration}),redirect:'error',signal:AbortSignal.timeout(2000)})}catch{}
  return {verification:'native-artifact-smoke',target:m.target,supported:m.supported,sourceFingerprint:m.sourceFingerprint,limitations:['clean-profile/default-browser/vendor-host/signing not certified']};
 }finally{if(child&&child.exitCode===null){child.kill();await exited}await cleanup(root,'roooooom-bundle-private-')}
}
export async function verifyBundle(bundle,{staticOnly=false}={}){
 bundle=resolve(bundle);const stat=await lstat(bundle);if(!stat.isDirectory()||stat.isSymbolicLink())throw Error('Regular bundle directory required');
 const m=await json(join(bundle,'manifest.json'));
 if(m.license!=='GPL-3.0-only'||!m.files?.LICENSE||!m.files?.COPYRIGHT)throw Error('Project GPL license missing');
 if(m.format!==1||m.product!=='Roooooom'||m.protocolVersion!==1||m.schemaVersion!==2||m.cgoEnabled!==false||!sha.test(m.sourceFingerprint)||!sha.test(m.apiHash)||!m.files||!m.uiFiles?.['index.html']||!Object.keys(m.uiFiles).some(p=>p.startsWith('assets/')&&p.endsWith('.js')))throw Error('Incomplete standalone/UI manifest');
 if(m.supported&&(!m.platform?.nativeEvidenceVerified||!m.platform?.cleanProfileVerified))throw Error('Unverified platform advertised');
 if(m.signing!=='unsigned'&&m.signing!=='unsigned-not-notarized')throw Error('Signing claim is not supported by this local build');
 const names=await files(bundle),expected=[...Object.keys(m.files),'manifest.json'].sort();if(JSON.stringify(names)!==JSON.stringify(expected))throw Error('Unexpected or missing bundle files');
 for(const [path,sum] of Object.entries(m.files)){if(!relativeFile(path)||!sha.test(sum)||/(?:^|\/)(?:node_modules|\.git|credentials|clients)(?:\/|$)/.test(path)||/(?:\.sqlite(?:-wal|-shm)?|\.env(?:\..*)?|control\.json|activation\.html)$/.test(path))throw Error('Forbidden runtime/private member');if(hash(await readFile(join(bundle,path)))!==sum)throw Error('Bundle checksum differs: '+path)}
 for(const [path,sum] of Object.entries(m.uiFiles))if(!relativeFile(path)||!sha.test(sum))throw Error('Invalid UI inventory');
 for(const path of [coreName(m.target),launcherName(m.target),'skills/roooooom/SKILL.md','api/openapi.yaml','dependencies.json','THIRD_PARTY_NOTICES.md','PLATFORMS.json','SHA256SUMS'])if(!(path in m.files))throw Error('Required bundle member missing: '+path);
 if(hash((await readFile(join(bundle,'api/openapi.yaml'),'utf8')).replaceAll('\r\n','\n'))!==m.apiHash)throw Error('Companion API differs');
 if(staticOnly)return {verification:'static-only',target:m.target,supported:m.supported,nativeExecution:'unverified'};
 if(m.target!==nativeTarget())throw Error('Native execution cannot verify another platform; use --static');
 return nativeSmoke(bundle,m);
}
async function rewriteFixtureManifest(root){const m=await json(join(root,'manifest.json'));const all=await inventory(root);delete all['manifest.json'];delete all.SHA256SUMS;await writeFile(join(root,'SHA256SUMS'),Object.entries(all).map(([p,h])=>h+'  '+p+'\n').join(''));m.files=await inventory(root);delete m.files['manifest.json'];await writeJSON(join(root,'manifest.json'),m)}
async function selfTest(bundle){
 await verifyBundle(bundle);
 const root=await mkdtemp(join(tmpdir(),'roooooom-bundle-counterexample-'));
 try{
  const noLicense=join(root,'no-license');await mkdir(noLicense);for(const p of await files(bundle)){if(p==='LICENSE')continue;await mkdir(dirname(join(noLicense,p)),{recursive:true});await copyFile(join(bundle,p),join(noLicense,p))}
  await rewriteFixtureManifest(noLicense);
  let licenseRejected=false;try{await run(process.execPath,[entry,'--bundle',noLicense],{capture:true})}catch(e){assert.match(e.message,/Project GPL license missing/);licenseRejected=true}assert.equal(licenseRejected,true,'A self-consistent package missing the project license was accepted');
  const copy=join(root,'no-ui');await mkdir(copy);for(const p of await files(bundle)){await mkdir(dirname(join(copy,p)),{recursive:true});await copyFile(join(bundle,p),join(copy,p))}
  const m=await json(join(bundle,'manifest.json')),version=await binaryJSON(join(bundle,coreName(m.target)),['version','--json'],environment(root));
  const src=join(root,'source');await mkdir(join(src,'cmd/roooooom'),{recursive:true});await writeFile(join(src,'go.mod'),'module roooooom.local/roooooom\n\ngo 1.27.0\n\ntoolchain go1.27.3\n');
  const code='package main\nimport("os";"flag";"net";"net/http";"path/filepath";"encoding/json")\nfunc main(){if len(os.Args)>1&&os.Args[1]=="version"{os.Stdout.Write([]byte('+JSON.stringify(JSON.stringify(version))+'));return};f:=flag.NewFlagSet("serve",flag.ExitOnError);root:=f.String("data-dir","","");listen:=f.String("listen","127.0.0.1:0","");f.Parse(os.Args[2:]);ln,e:=net.Listen("tcp",*listen);if e!=nil{panic(e)};os.MkdirAll(*root,0700);d:=map[string]any{"listenAddr":ln.Addr().String(),"serviceId":"no-ui-fixture","datasetGeneration":"fixture","protocolVersion":1};b,_:=json.Marshal(d);os.WriteFile(filepath.Join(*root,"discovery.json"),b,0600);mux:=http.NewServeMux();mux.HandleFunc("/identity",func(w http.ResponseWriter,r *http.Request){json.NewEncoder(w).Encode(map[string]any{"status":"ok","serviceId":"no-ui-fixture","datasetGeneration":"fixture","protocolVersion":1})});http.Serve(ln,mux)}\n';
  await writeFile(join(src,'cmd/roooooom/main.go'),code);
  await run('go',['build','-trimpath','-buildvcs=false','-o',join(copy,coreName(m.target)),'./cmd/roooooom'],{cwd:src,env:{CGO_ENABLED:'0'}});
  await rewriteFixtureManifest(copy);
  let rejected=false;try{await run(process.execPath,[entry,'--bundle',copy],{capture:true})}catch(e){assert.match(e.message,/embedded UI missing/);rejected=true}assert.equal(rejected,true,'A native binary deliberately built without Web UI was accepted');
  const noProof=join(root,'empty-source');await mkdir(noProof);await assert.rejects(webProof(noProof),/Fresh Web embed proof missing/);
  console.log('Bundle counterexamples passed: missing project license and real no-UI binary rejected by the normal verification entry.');
 }finally{await cleanup(root,'roooooom-bundle-counterexample-')}
}
const a=args();if(typeof a['--bundle']!=='string')throw Error('--bundle required');const bundle=resolve(a['--bundle']);
if(a['--self-test'])await selfTest(bundle);else console.log(JSON.stringify(await verifyBundle(bundle,{staticOnly:!!a['--static']})));
