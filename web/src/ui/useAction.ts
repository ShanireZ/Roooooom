import {useRef,useState} from 'react';
import {ApiError} from '../api/client';

// One in-flight write per control; errors keep the server code for Feedback.
export function useAction(){
 const busyRef=useRef(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
 const run=async<T,>(work:()=>Promise<T>):Promise<T|undefined>=>{
  if(busyRef.current)return;
  busyRef.current=true;setBusy(true);setError('');setNotice('');
  try{return await work()}
  catch(e){setError(e instanceof ApiError?e.code:'unknown')}
  finally{busyRef.current=false;setBusy(false)}
 };
 return {busy,error,notice,setNotice,setError,run};
}
