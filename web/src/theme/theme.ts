export type ThemePreference='system'|'light'|'dark';
// A per-browser convenience; it never affects coordination state.
const KEY='roooooom.theme';
export function readTheme():ThemePreference{
 try{const value=localStorage.getItem(KEY);return value==='light'||value==='dark'?value:'system'}
 catch{return 'system'}
}
export function applyTheme(preference:ThemePreference){
 document.documentElement.dataset.theme=preference;
 try{localStorage.setItem(KEY,preference)}catch{/* storage may be unavailable */}
}
