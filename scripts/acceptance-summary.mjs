import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve, dirname, basename } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const entry = fileURLToPath(import.meta.url);
export const scenarioIds = Array.from({length:36}, (_, i) => 'A'+String(i+1).padStart(2,'0'));
const hostCases = new Set(['A01','A04','A05','A07','A08','A11','A12','A13','A14','A20','A22','A23','A25','A35']);
const externalCases = new Set(['A29','A33']);
export const requirements = Object.fromEntries(scenarioIds.map(id => [id, [hostCases.has(id)?'real-host':externalCases.has(id)?'real-external':id==='A36'?'real-platform':'synthetic']]));

export function summarize(report) {
  const issues=[], missing=[], valid=new Map(), witnesses=new Map(), hosts=new Map();
  const text=v=>typeof v==='string'&&v.trim().length>0&&v.length<=2000;
  const hex=(v,n)=>typeof v==='string'&&new RegExp('^[a-f0-9]{'+n+'}$','i').test(v);
  const list=Array.isArray, object=v=>v!==null&&typeof v==='object'&&!list(v);
  const ref=v=>typeof v==='string'&&/^evidence:[a-z0-9][a-z0-9._-]{0,127}$/i.test(v);
  const ids=v=>list(v)&&v.every(x=>typeof x==='string'&&/^[a-z0-9][a-z0-9._-]{0,127}$/i.test(x));
  const bad=message=>issues.push(message);
  if(!object(report))return {accepted:false,m3Accepted:false,verification:'self-reported-completeness-only',missing:scenarioIds,issues:['Report must be an object.']};
  // A heuristic prevents common accidental exports. It is not a guarantee of deidentification.
  if(/(?:ghp_|github_pat_|Bearer\s|(?:token|secret|access_token|sign)=|[a-z]:[\\/]|\/(?:Users|home)\/)/i.test(JSON.stringify(report)))bad('Possible private value/path; content withheld.');
  if(report.format!==1||!hex(report.commit,40))bad('Invalid format/commit.');
  for(const key of ['go','node','npm'])if(!text(report.toolchain?.[key]))bad('Missing toolchain '+key+'.');
  for(const key of ['go.mod','go.sum','web/package-lock.json'])if(!hex(report.lockHashes?.[key],64))bad('Missing lock hash '+key+'.');
  for(const key of ['os','arch','cpu','browser'])if(!text(report.environment?.[key]))bad('Missing environment '+key+'.');
  const b=report.bundle;
  if(!object(b)||!hex(b.sha256,64)||!hex(b.sourceFingerprint,64)||b.commit!==report.commit||b.dirty!==false||!text(b.target))bad('Bundle must match the clean recorded commit and include hashes/target.');
  if(!list(report.hosts)||report.hosts.length>100)bad('Invalid host inventory.');
  for(const h of list(report.hosts)?report.hosts:[]) {
    if(!object(h)||!ids([h.id])||hosts.has(h.id)||!['vendor','host','version'].every(k=>text(h[k]))||!Number.isFinite(h.toolTimeoutSeconds)||h.toolTimeoutSeconds<=0)bad('Invalid/duplicate host inventory entry.');
    else hosts.set(h.id,h);
  }
  const origins={'synthetic':['test-run','fixture','mock'],'real-host':['host-tool-call'],'real-external':['platform-delivery'],'real-platform':['os-observation']};
  if(!list(report.witnesses)||report.witnesses.length>2000)bad('Invalid witness inventory.');
  for(const w of list(report.witnesses)?report.witnesses:[]) {
    if(!object(w)||!ids([w.id])||witnesses.has(w.id)||!origins[w.source]?.includes(w.origin)||!ref(w.reference)||!hex(w.sha256,64)||!text(w.description)||w.commit!==report.commit||w.bundleHash!==b?.sha256||!ids(w.hostIds)||w.hostIds.some(id=>!hosts.has(id)))bad('Invalid/duplicate witness or evidence class/artifact mismatch.');
    else witnesses.set(w.id,w);
  }
  if(!list(report.cases)||report.cases.length>400)bad('Invalid cases.');
  const seenCases=new Set();
  for(const c of list(report.cases)?report.cases:[]) {
    if(!object(c)||!scenarioIds.includes(c.scenarioId)||!Object.hasOwn(origins,c.source)){bad('Unknown case or evidence class.');continue;}
    const key=c.scenarioId+':'+c.source;
    if(seenCases.has(key)){bad('Duplicate case '+key+'.');continue;}seenCases.add(key);
    if(!['passed','failed','skip','unverified'].includes(c.result)||!list(c.limitations)||!c.limitations.length||!c.limitations.every(text)||!ids(c.witnessIds)||!c.witnessIds.length||new Set(c.witnessIds).size!==c.witnessIds.length){bad('Incomplete case '+key+'.');continue;}
    if(c.result!=='passed')continue;
    const ws=c.witnessIds.map(id=>witnesses.get(id));
    if(ws.some(w=>!w||w.source!==c.source)){bad('Missing/mismatched witness '+key+'.');continue;}
    if(c.source==='real-host') {
      if(!ids(c.hostIds)||!c.hostIds.length||c.hostIds.some(id=>!hosts.has(id))||c.hostIds.some(id=>!ws.some(w=>w.hostIds.includes(id)))){bad('Missing real-host call witness '+key+'.');continue;}
      if(['A01','A20'].includes(c.scenarioId)&&new Set(c.hostIds.map(id=>hosts.get(id).vendor.normalize('NFKC').trim().toLowerCase())).size<2){bad('Two distinct vendors required for '+key+'.');continue;}
    }
    if(c.source==='real-external'&&(!object(c.external)||c.external.provider!==(c.scenarioId==='A29'?'github':'dingtalk')||!text(c.external.targetAlias)||!ref(c.external.authorizationRef))){bad('Missing scoped external authorization/reference '+key+'.');continue;}
    if(c.scenarioId==='A20') {
      const i=c.integration;
      if(!object(i)||!hex(i.baseHead,40)||!hex(i.integratedHead,40)||!list(i.deliveryHeads)||i.deliveryHeads.length<2||!i.deliveryHeads.every(h=>hex(h,40))||!ids(i.checkWitnessIds)||!i.checkWitnessIds.length||i.checkWitnessIds.some(id=>!c.witnessIds.includes(id))){bad('Incomplete integration heads/check witness A20.');continue;}
    }
    if(c.source==='real-platform') {
      const p=report.platform;
      if(!object(p)||p.target!==b?.target||p.cleanProfileVerified!==true||!text(p.minimumOS)||!list(p.systemLibraries)||!p.systemLibraries.length||!p.systemLibraries.every(text)||!text(p.signing)){bad('Clean platform evidence is unverified.');continue;}
    }
    valid.set(key,true);
  }
  for(const [id,sources] of Object.entries(requirements))for(const source of sources)if(!valid.has(id+':'+source))missing.push(id+':'+source);
  return {accepted:issues.length===0&&missing.length===0,m3Accepted:issues.length===0&&missing.every(x=>externalCases.has(x.slice(0,3))),verification:'self-reported-completeness-only',
    missing,issues,declaredPassed:[...valid.keys()].sort(),externalPending:missing.filter(x=>externalCases.has(x.slice(0,3))),
    limitation:'This checks declarations and references, not the truth of host behavior, platform delivery or human observations. Review the private witness artifacts.'};
}

// These are schema fixtures. They never claim that this test operated real hosts/platforms.
function completeFixture() {
  const sha = 'a'.repeat(64), commit = 'b'.repeat(40);
  const r = {format:1, commit, toolchain:{go:'go1.27.3', node:'v24.22.1', npm:'12.2.1'},
    lockHashes:{'go.mod':sha,'go.sum':sha,'web/package-lock.json':sha},
    environment:{os:'Windows 11',arch:'amd64',cpu:'redacted CPU model',browser:'Chromium'},
    bundle:{target:'windows-amd64',sha256:sha,sourceFingerprint:sha,commit,dirty:false},
    hosts:[{id:'host-a',vendor:'vendor-a',host:'host-cli-a',version:'1',toolTimeoutSeconds:60},
      {id:'host-b',vendor:'vendor-b',host:'host-ide-b',version:'2',toolTimeoutSeconds:90}],
    platform:{target:'windows-amd64',cleanProfileVerified:true,minimumOS:'Windows 11',systemLibraries:['OS supplied'],signing:'unsigned'},
    witnesses:[],cases:[]};
  for (const id of scenarioIds) {
    const source=requirements[id][0], wid=id.toLowerCase();
    const w={id:wid,source,origin:source==='synthetic'?'test-run':source==='real-host'?'host-tool-call':source==='real-external'?'platform-delivery':'os-observation',reference:'evidence:'+wid,sha256:sha,commit,bundleHash:sha,description:'deidentified call and output observation',hostIds:source==='real-host'?['host-a','host-b']:[]};
    const c={scenarioId:id,source,result:'passed',witnessIds:[wid],limitations:['Self-reported evidence requires human review.'],hostIds:w.hostIds};
    if(source==='real-external')c.external={provider:id==='A29'?'github':'dingtalk',targetAlias:'approved-target',authorizationRef:'evidence:human-authorization'};
    if(id==='A20')c.integration={baseHead:commit,deliveryHeads:['c'.repeat(40),'d'.repeat(40)],integratedHead:'e'.repeat(40),checkWitnessIds:[wid]};
    r.witnesses.push(w);r.cases.push(c);
  }
  return r;
}
async function selfTest() {
  const root=await mkdtemp(join(tmpdir(),'roooooom-acceptance-schema-'));
  try {
    const complete=completeFixture();
    const fixtures=[['complete',complete,true],
      ['missing-a20',{...structuredClone(complete),cases:complete.cases.filter(c=>c.scenarioId!=='A20')},false]];
    const mock=structuredClone(complete);mock.witnesses.find(w=>w.id==='a20').origin='mock';fixtures.push(['mock-as-real-host',mock,false]);
    const platform=structuredClone(complete);platform.platform.cleanProfileVerified=false;fixtures.push(['unverified-platform',platform,false]);
    const absent=structuredClone(complete);absent.witnesses=absent.witnesses.filter(w=>w.id!=='a20');fixtures.push(['missing-witness',absent,false]);
    const skip=structuredClone(complete);skip.cases.find(c=>c.scenarioId==='A20').result='skip';fixtures.push(['skip',skip,false]);
    const vendor=structuredClone(complete);vendor.hosts[1].vendor=vendor.hosts[0].vendor;fixtures.push(['one-vendor',vendor,false]);
    const old=structuredClone(complete);old.bundle.commit='f'.repeat(40);fixtures.push(['other-artifact',old,false]);
    const secret=structuredClone(complete);secret.cases[0].limitations=['token=must-not-be-in-report'];fixtures.push(['private-value',secret,false]);
    for (const [name,report,expected] of fixtures) {
      const path=join(root,name+'.json');await writeFile(path,JSON.stringify(report));
      const p=spawnSync(process.execPath,[entry,'--report',path],{encoding:'utf8',timeout:10000,windowsHide:true});
      assert.ifError(p.error);const s=JSON.parse(p.stdout);
      assert.equal(s.accepted,expected,name+': actual summary entry accepted wrong evidence');
      assert.equal(p.status,expected?0:1,name+': wrong exit status');
      if(name==='missing-a20')assert(s.missing.some(x=>x.startsWith('A20')));
    }
    console.log('Acceptance summary: '+fixtures.length+' actual-entry schema fixtures passed; no real-host/external/platform evidence was produced.');
  } finally {
    assert.equal(dirname(root),resolve(tmpdir()));assert(basename(root).startsWith('roooooom-acceptance-schema-'));
    await rm(root,{recursive:true,force:true});
  }
}

if (process.argv[1] && resolve(process.argv[1])===entry) {
  if(process.argv.length===3&&process.argv[2]==='--self-test')await selfTest();
  else if(process.argv.length===4&&process.argv[2]==='--report') {
    try {const r=summarize(JSON.parse(await readFile(process.argv[3],'utf8')));console.log(JSON.stringify(r,null,2));process.exitCode=r.accepted?0:1;}
    catch {console.log(JSON.stringify({accepted:false,m3Accepted:false,missing:scenarioIds,issues:['Invalid or unreadable report; raw content withheld.']}));process.exitCode=1;}
  }else {console.error('Usage: node scripts/acceptance-summary.mjs --report PRIVATE_REPORT.json | --self-test');process.exitCode=2;}
}
