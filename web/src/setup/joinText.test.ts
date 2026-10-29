import {describe,expect,it} from 'vitest';
import {createTranslator,LOCALES} from '../i18n/translator';
import {joinInstructions} from './joinText';

const project={id:'proj-123',name:'SimpleProject',repositories:[{id:'r1',name:'main',reference:'C:\\Projects\\SimpleProject'}]} as Parameters<typeof joinInstructions>[1]['project'];
const info={clientPath:'C:\\Program Files\\Roooooom\\roooooom.exe',skillPath:'C:\\Program Files\\Roooooom\\skills\\roooooom\\SKILL.md',dataDir:'C:\\ProgramData\\Roooooom\\data',serviceURL:'http://127.0.0.1:7337'};

describe('joinInstructions',()=>{
 it('contains the project id, skill path and connect command in every locale, without backslashes',()=>{
  for(const locale of LOCALES){
   const text=joinInstructions(createTranslator(locale).t,{project,info});
   expect(text,locale).toContain('proj-123');
   expect(text,locale).toContain('C:/Program Files/Roooooom/skills/roooooom/SKILL.md');
   expect(text,locale).toContain('connect --data-dir');
   expect(text,locale).toContain('--project proj-123');
   expect(text,locale).toContain('SimpleProject');
   expect(text,locale).not.toContain('\\');
   expect(text,locale).not.toContain('[missing:');
  }
 });
});
