import {routePath} from './routes';

// The last visited room is a per-browser convenience, never coordination state.
const LAST_ROOM='roooooom.lastRoom';
export function readLastRoom():string|null{
 try{return localStorage.getItem(LAST_ROOM)}catch{return null}
}
export function rememberRoom(projectId:string){
 try{localStorage.setItem(LAST_ROOM,projectId)}catch{/* storage may be unavailable */}
}

// `/` goes to the last visited room, then the first room with owner items,
// then the first room by creation order (rooms arrive in that order), else room creation.
export function homeTarget(rooms:readonly {id:string}[],items:readonly {projectId:string}[],last:string|null):string{
 const room=rooms.find(r=>r.id===last)??rooms.find(r=>items.some(i=>i.projectId===r.id))??rooms[0];
 return room?routePath({name:'room',projectId:room.id}):routePath({name:'newRoom'});
}
