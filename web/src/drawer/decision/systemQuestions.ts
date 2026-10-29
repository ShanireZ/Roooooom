import type {Schema} from '../../api/client';
import type {Translator} from '../../i18n/translator';

export type LocalizedQuestion={question:string;reason:string;options:{id:string;label:string;impact:string}[];recommendationReason:string;canContinue:string[];noDecisionBehavior:string};
const RISK_OPTIONS=['authorize-bounded','revise-first'];

// System-authored questions carry a messageKey and render in the reader's language;
// agent-authored questions are always shown verbatim.
export function localizeSystemQuestion(q:Schema['DecisionRequestView'],t:Translator['t']):LocalizedQuestion{
 const stored:LocalizedQuestion={question:q.question,reason:q.reason,options:q.options.map(o=>({id:o.id,label:o.label,impact:o.impact})),recommendationReason:q.recommendationReason,canContinue:q.canContinue,noDecisionBehavior:q.noDecisionBehavior};
 if(q.messageKey!=='system.riskStart')return stored;
 return {question:t('sysq.risk.question'),reason:t('sysq.risk.reason'),recommendationReason:t('sysq.risk.recommendationReason'),canContinue:[t('sysq.risk.canContinue')],noDecisionBehavior:t('sysq.risk.noDecision'),
  options:stored.options.map(o=>RISK_OPTIONS.includes(o.id)?{id:o.id,label:t(('sysq.risk.option.'+o.id+'.label') as 'sysq.risk.question'),impact:t(('sysq.risk.option.'+o.id+'.impact') as 'sysq.risk.question')}:o)};
}
