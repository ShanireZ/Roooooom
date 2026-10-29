import {useEffect,useId,useRef,useState,type ReactNode} from 'react';
import './drawer.css';
import {useI18n} from '../i18n/I18nProvider';
import {IconClose} from '../ui/icons';
import {DURATION,reducedMotion} from '../ui/motion';

// One drawer at a time. Wide screens get a non-modal side panel so the floor stays
// usable; narrow screens get a full-screen modal. Escape and the close button call
// onClose after a short exit motion, and focus returns to whatever opened the drawer.
export function Drawer({title,onClose,children}:{title:string;onClose:()=>void;children:ReactNode}){
 const {t}=useI18n(),dialog=useRef<HTMLDialogElement>(null),heading=useId();
 const opener=useRef<Element|null>(typeof document==='undefined'?null:document.activeElement);
 const [closing,setClosing]=useState(false),onCloseRef=useRef(onClose),leaving=useRef(false);
 onCloseRef.current=onClose;
 const close=useRef(()=>{
  if(leaving.current)return;
  leaving.current=true;
  if(reducedMotion()){onCloseRef.current();return}
  setClosing(true);
  setTimeout(()=>onCloseRef.current(),DURATION.quick);
 });
 useEffect(()=>{
  const d=dialog.current;
  if(!d)return;
  if(!d.open){if(matchMedia('(min-width: 760px)').matches)d.show();else d.showModal()}
  d.querySelector<HTMLElement>('h2')?.focus();
  const key=(e:KeyboardEvent)=>{if(e.key==='Escape'){e.preventDefault();close.current()}};
  const cancel=(e:Event)=>{e.preventDefault();close.current()};
  d.addEventListener('keydown',key);d.addEventListener('cancel',cancel);
  return()=>{
   d.removeEventListener('keydown',key);d.removeEventListener('cancel',cancel);
   if(d.open)d.close();
   const back=opener.current;
   if(back instanceof HTMLElement&&document.contains(back))back.focus();
  };
 },[]);
 return <dialog ref={dialog} className="drawer" aria-labelledby={heading} data-closing={closing?'true':undefined}>
  <header className="drawer-head">
   <h2 id={heading} tabIndex={-1}>{title}</h2>
   <button type="button" className="btn-ghost drawer-close" onClick={()=>close.current()}><IconClose/><span className="sr-only">{t('drawer.close')}</span></button>
  </header>
  <div className="drawer-body">{children}</div>
 </dialog>;
}
