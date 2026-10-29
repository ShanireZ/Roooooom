import {useId,type InputHTMLAttributes,type ReactNode,type SelectHTMLAttributes,type TextareaHTMLAttributes} from 'react';

// Labelled controls; `identifier` disables autocorrect for paths and ids.
export function Field({label,identifier=false,...props}:InputHTMLAttributes<HTMLInputElement>&{label:string;identifier?:boolean}){
 const id=useId();
 return <div className="field"><label htmlFor={id}>{label}</label><input id={id} name={props.name??id} {...(identifier?{autoCorrect:'off',autoCapitalize:'off',spellCheck:false}:{})} {...props}/></div>;
}
export function Area({label,...props}:TextareaHTMLAttributes<HTMLTextAreaElement>&{label:string}){
 const id=useId();
 return <div className="field"><label htmlFor={id}>{label}</label><textarea id={id} name={props.name??id} {...props}/></div>;
}
export function Select({label,children,...props}:SelectHTMLAttributes<HTMLSelectElement>&{label:string;children:ReactNode}){
 const id=useId();
 return <div className="field"><label htmlFor={id}>{label}</label><select id={id} name={props.name??id} {...props}>{children}</select></div>;
}
export function Toggle({label,...props}:InputHTMLAttributes<HTMLInputElement>&{label:string}){
 const id=useId();
 return <label className="choice" htmlFor={id}><input id={id} name={props.name??id} type="checkbox" {...props}/><span>{label}</span></label>;
}
