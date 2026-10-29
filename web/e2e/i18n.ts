// Room specs run with locale en-US and read UI text from the English catalog.
import {createTranslator} from '../src/i18n/translator';
const en=createTranslator('en');
export const m=en.t;
export const mn=en.tn;
