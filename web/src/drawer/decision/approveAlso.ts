import type {Schema} from '../../api/client';

type Question=Schema['DecisionRequestView'];
type SpecAmendment=Schema['SpecAmendment'];
type ExternalAuthorization=Schema['ExternalAuthorization'];
export type ApproveAlso={spec?:{prefill:SpecAmendment;proposed:boolean};external?:{prefill:Partial<ExternalAuthorization>;proposed:boolean}};

// Mirrors the server's acceptance rules (decisions.go): what a question blocks decides
// which extra approvals it can carry; an agent proposal only prefills the fields.
export function approveAlso(q:Question,task?:Schema['Task']):ApproveAlso{
 const result:ApproveAlso={};
 if(q.blockedActions.includes('spec-change')&&task){
  result.spec=q.specProposal?{prefill:q.specProposal,proposed:true}:{prefill:{goal:task.goal,scope:task.scope,nonScope:task.nonScope,authorityRefs:task.authorityRefs,risk:task.risk},proposed:false};
 }
 if(q.blockedActions.includes('external-action')){
  result.external=q.externalProposal?{prefill:q.externalProposal,proposed:true}:{prefill:{},proposed:false};
 }
 return result;
}
