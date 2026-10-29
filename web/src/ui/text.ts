// One item per non-empty line, trimmed: the form convention for lists.
export const lines=(text:FormDataEntryValue|null|undefined)=>String(text??'').split(/\r?\n/).map(x=>x.trim()).filter(Boolean);
