// A floor-plan mark: a room outline with a door swing, drawn at the rule weight.
export function Wordmark({href='/'}:{href?:string}){
 return <a className="wordmark" href={href} data-local>
  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true"><path d="M3 3h18v18H11"/><path d="M3 3v18h3"/><path d="M6 21a5 5 0 0 1 5-5"/></svg>
  <span>ROOOOOOM</span>
 </a>;
}
