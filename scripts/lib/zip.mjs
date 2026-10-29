import {deflateRawSync} from 'node:zlib';
import {readFile,writeFile} from 'node:fs/promises';
import {join} from 'node:path';
import {files} from './packaging.mjs';
const table=Array.from({length:256},(_,i)=>{let n=i;for(let k=0;k<8;k++)n=n&1?0xedb88320^(n>>>1):n>>>1;return n>>>0});
const crc32=b=>{let crc=0xffffffff;for(const x of b)crc=table[(crc^x)&255]^(crc>>>8);return (crc^0xffffffff)>>>0};
export async function zip(root,destination,executables){
 const locals=[],central=[];let offset=0,count=0;
 for(const path of await files(root)){
  if(path.startsWith('/')||path.includes('..')||path.includes('\\'))throw Error('Unsafe archive member');
  const name=Buffer.from(path),raw=await readFile(join(root,path)),compressed=deflateRawSync(raw,{level:9}),crc=crc32(raw);
  if(raw.length>=0xffffffff||compressed.length>=0xffffffff)throw Error('ZIP64 is not supported');
  const local=Buffer.alloc(30);local.writeUInt32LE(0x04034b50,0);local.writeUInt16LE(20,4);local.writeUInt16LE(0x800,6);local.writeUInt16LE(8,8);local.writeUInt16LE(0x21,12);local.writeUInt32LE(crc,14);local.writeUInt32LE(compressed.length,18);local.writeUInt32LE(raw.length,22);local.writeUInt16LE(name.length,26);
  const c=Buffer.alloc(46);c.writeUInt32LE(0x02014b50,0);c.writeUInt16LE(0x314,4);c.writeUInt16LE(20,6);c.writeUInt16LE(0x800,8);c.writeUInt16LE(8,10);c.writeUInt16LE(0x21,14);c.writeUInt32LE(crc,16);c.writeUInt32LE(compressed.length,20);c.writeUInt32LE(raw.length,24);c.writeUInt16LE(name.length,28);c.writeUInt32LE((((executables.includes(path)?0o100755:0o100644)<<16)|0x20)>>>0,38);c.writeUInt32LE(offset,42);
  locals.push(local,name,compressed);central.push(c,name);offset+=local.length+name.length+compressed.length;count++;
 }
 const directory=Buffer.concat(central),end=Buffer.alloc(22);end.writeUInt32LE(0x06054b50,0);end.writeUInt16LE(count,8);end.writeUInt16LE(count,10);end.writeUInt32LE(directory.length,12);end.writeUInt32LE(offset,16);
 await writeFile(destination,Buffer.concat([...locals,directory,end]),{flag:'wx'});
}
