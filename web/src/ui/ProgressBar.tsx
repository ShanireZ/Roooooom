// One segment per assignment; the table's accessible name carries the meaning.
export type Segment='accepted'|'active'|'owner'|'blocked'|'open';
export function ProgressBar({segments}:{segments:Segment[]}){
 return <span className="bar" aria-hidden="true">{segments.map((s,i)=><i key={i} className={'seg-'+s}/>)}</span>;
}
