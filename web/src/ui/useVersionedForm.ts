import {useState} from 'react';

// A management form binds the version shown when it opened. When the record
// changes underneath, the form is stale until the owner reopens it (spec §8.1).
export function useVersionedForm<T>(current:T,versionOf:(x:T)=>number){
 const [base,setBase]=useState(current);
 return {base,stale:versionOf(current)!==versionOf(base),reopen:()=>setBase(current)};
}
