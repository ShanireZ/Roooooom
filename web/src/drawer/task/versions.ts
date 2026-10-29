import type {Schema} from '../../api/client';

// The version fences every task-scoped owner write carries.
export function versions(task:Schema['Task']){
 return {expectedVersion:task.version,integratorEpoch:task.integratorEpoch,specVersion:task.specVersion,acceptancePolicyVersion:task.acceptancePolicyVersion,decisionVersions:task.decisionVersions};
}
