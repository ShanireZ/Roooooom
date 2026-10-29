import type {Schema} from '../api/client';
import type {Translator} from '../i18n/translator';

export type ConnectionInfo={clientPath:string;skillPath:string;dataDir:string;serviceURL:string};
const slash=(path:string)=>path.replaceAll('\\','/');

// Non-secret instructions the owner pastes into the original agent conversation.
// Paths and identifiers stay verbatim; only the surrounding words are localized.
export function joinInstructions(t:Translator['t'],{project,info}:{project:Schema['ProjectView'];info:ConnectionInfo}):string{
 return [
  t('join.intro'),
  t('join.project',{name:project.name}),
  t('join.repos',{refs:project.repositories.map(r=>slash(r.reference)).join('; ')}),
  t('join.skill',{path:slash(info.skillPath)}),
  t('join.cli',{path:slash(info.clientPath)}),
  t('join.dataRoot',{path:slash(info.dataDir)}),
  t('join.service',{url:info.serviceURL}),
  t('join.projectId',{id:project.id}),
  t('join.connect',{id:project.id}),
  t('join.sync'),
  t('join.guide'),
 ].join('\n');
}
