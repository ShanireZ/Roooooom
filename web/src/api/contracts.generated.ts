// Generated from api/openapi.yaml. Do not edit.
export interface paths {
    "/api/v1/setup/activate": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description One-time five-minute private file form. Only this route accepts Origin null. */
        post: operations["activateOwner"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/me": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["currentActor"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/enroll": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["enrollAgent"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/commands/{kind}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Owner configuration requires owner cookie, exact Origin and X-CSRF-Token. Body actor fields are forbidden. */
        post: operations["applyCommand"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/health": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["health"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/queries/{kind}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description needs-you and activity are owner-only room reads; assignments accepts projectId without targetId to read every open task's assignments. */
        get: operations["readWorkspace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sessions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Creates a fresh session for the same participant and project; transfers no responsibilities. */
        post: operations["createWorkSession"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/identity": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["identity"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/sync": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["syncWorkspace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/wait": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["waitWorkspace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/events": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["eventsWorkspace"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/control/probe": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Private native launcher control plane. Raw bounded response, outside agent commands/events; exact identity and installation checks. No vendor-host control. */
        post: operations["ownServiceProbe"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/control/activate": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Private native launcher control plane. Raw bounded response, outside agent commands/events; exact identity and installation checks. No vendor-host control. */
        post: operations["ownServiceActivate"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/control/stop": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Private native launcher control plane. Raw bounded response, outside agent commands/events; exact identity and installation checks. No vendor-host control. */
        post: operations["ownServiceStop"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/setup/connection": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description Owner-only product paths for generated connection instructions. Never returns capability/token. */
        get: operations["ownerConnectionInstructions"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        Mutation: {
            serviceId: string;
            datasetGeneration: string;
            clientRequestId: string;
            targetId?: string;
            /** Format: int64 */
            expectedVersion?: number;
            /** Format: int64 */
            claimEpoch?: number;
            /** Format: int64 */
            integratorEpoch?: number;
            /** Format: int64 */
            specVersion?: number;
            /** Format: int64 */
            acceptancePolicyVersion?: number;
            sessionId?: string;
            decisionVersions?: {
                [key: string]: number;
            };
            payload: components["schemas"]["ProjectCreate"] | components["schemas"]["Empty"] | components["schemas"]["TaskCreate"] | components["schemas"]["AssignmentCreate"] | components["schemas"]["Release"] | components["schemas"]["AssignmentUpdate"] | components["schemas"]["Limit"] | components["schemas"]["Reason"] | components["schemas"]["Checkpoint"] | components["schemas"]["Resume"] | components["schemas"]["Handoff"] | components["schemas"]["MessageSend"] | components["schemas"]["ReceiptUpdate"] | components["schemas"]["DecisionRequest"] | components["schemas"]["DecisionResolve"] | components["schemas"]["DecisionFollowup"] | components["schemas"]["DeliverySubmit"] | components["schemas"]["DeliveryReview"] | components["schemas"]["IntegrationRecord"] | components["schemas"]["AcceptancePolicyChange"] | components["schemas"]["AcceptanceCandidate"] | components["schemas"]["AcceptCandidate"] | components["schemas"]["OfflineReconcile"] | components["schemas"]["OfflineReview"] | components["schemas"]["RequestCreate"] | components["schemas"]["RequestHandoff"] | components["schemas"]["RequestResume"] | components["schemas"]["RequestAssign"] | components["schemas"]["RequestConvert"] | components["schemas"]["BindingCredential"] | components["schemas"]["BindingInspectionInput"];
        };
        Checkpoint: {
            summary: string;
            handoffTargetSessionId?: string;
            responsibilityIds?: string[];
        };
        Resume: {
            expectedSourceSessionId: string;
            /** Format: int64 */
            sourceVersion: number;
            responsibilityIds: string[];
            /** @description Private managed file reference */
            handoffCapabilityRef?: string;
            /** @description Owner only */
            force?: boolean;
            reason?: string;
        };
        Handoff: {
            fromSessionId: string;
            toSessionId: string;
            handoffId?: string;
            /** Format: int64 */
            acceptedHandoffVersion?: number;
            dispositions: {
                assignmentId: string;
                /** @enum {string} */
                action: "transfer" | "release" | "continue-worker";
                targetSessionId?: string;
            }[];
            reason: string;
            /** @description Owner only */
            force?: boolean;
        };
        AcceptancePolicy: {
            /** @enum {string} */
            mode: "integrator" | "designated-agent" | "human";
            acceptorId?: string;
            conditions: string[];
            ownerProtected: boolean;
        };
        TaskCreate: {
            projectId: string;
            goal: string;
            scope: string[];
            nonScope: string[];
            authorityRefs: string[];
            /** @enum {string} */
            risk: "low" | "normal" | "high";
            acceptancePolicy: components["schemas"]["AcceptancePolicy"];
            collaboratorLimit: number;
            designatedIntegratorId?: string;
        };
        Task: {
            projectId: string;
            goal: string;
            scope: string[];
            nonScope: string[];
            authorityRefs: string[];
            /** @enum {string} */
            risk: "low" | "normal" | "high";
            acceptancePolicy: components["schemas"]["AcceptancePolicy"];
            collaboratorLimit: number;
            designatedIntegratorId?: string;
            id: string;
            /** Format: int64 */
            version: number;
            /** Format: int64 */
            specVersion: number;
            /** Format: int64 */
            acceptancePolicyVersion: number;
            createdBy: string;
            /** @enum {string} */
            actorKind: "human" | "agent";
            integratorId?: string;
            integratorSessionId?: string;
            /** Format: int64 */
            integratorEpoch: number;
            /** @enum {string} */
            phase: "draft" | "ready" | "active" | "integrating" | "review" | "done" | "cancelled";
            canStart: boolean;
            collaboratorCount: number;
            attention: string[];
            acceptanceConstraints: components["schemas"]["AcceptanceConstraints"];
            currentCandidateId?: string;
            latestIntegrationId?: string;
            lastCompletionId?: string;
            decisionVersions: {
                [key: string]: number;
            };
            participantIds?: string[];
            readonly recordRefs?: {
                bindingId: string;
                number: string;
                url: string;
                taskVersion: number;
            }[];
            sourceRequestId?: string;
            sourceRequestText?: string;
            sourceConstraints?: string[];
            /** @enum {string} */
            collaboratorLimitSource?: "owner";
            /**
             * Format: date-time
             * @description Read-only, derived from events: first event about the task.
             */
            createdAt?: string;
            /**
             * Format: date-time
             * @description Read-only, derived from events: latest event about the task or its records.
             */
            updatedAt?: string;
            /**
             * Format: date-time
             * @description Read-only, derived from events: completion time; present only when phase is done.
             */
            completedAt?: string;
        };
        AssignmentCreate: {
            taskId: string;
            role: string;
            input: string;
            output: string;
            writeScope: string[];
            repoId: string;
            baseRef: string;
            dependencies: string[];
        };
        /** @description Cancelled requires owner/current integrator, exact assignment version and reason, without contract edits; preserves immutable deliveries and leaves dependent assignments blocked. */
        AssignmentUpdate: {
            /** @enum {string} */
            state?: "active" | "blocked" | "cancelled";
            dependencies?: string[];
            role?: string;
            input?: string;
            output?: string;
            writeScope?: string[];
            repoId?: string;
            baseRef?: string;
            reason?: string;
        };
        Release: {
            reason: string;
            exitTask?: boolean;
        };
        Limit: {
            limit: number;
            reason: string;
        };
        Reason: {
            reason: string;
        };
        ProjectCreate: {
            name: string;
            repositories: {
                name: string;
                reference: string;
            }[];
        };
        Empty: Record<string, never>;
        Enrollment: {
            serviceId: string;
            datasetGeneration: string;
            clientRequestId: string;
            name: string;
        };
        Credentials: {
            participantId: string;
            sessionId: string;
            /** @description Secret; write only to private credential storage */
            token: string;
        };
        Envelope: {
            requestId: string;
            result: unknown;
            objectVersions: {
                [key: string]: number;
            };
            /** Format: int64 */
            committedEventId: number;
            unread: unknown[];
            cursor: string;
            hasMore: boolean;
            /** Format: date-time */
            serverTime: string;
            /** @constant */
            connection: "online";
        };
        Error: {
            error: {
                code: string;
                message: string;
            };
        };
        Identity: {
            /** @constant */
            status: "ok";
            serviceId: string;
            datasetGeneration: string;
            /** @constant */
            protocolVersion: 1;
        };
        MessageSend: {
            projectId: string;
            taskId?: string;
            assignmentId?: string;
            threadId: string;
            /** @enum {string} */
            purpose: "question" | "proposal" | "contract-change" | "handoff" | "delivery" | "decision-notice" | "status" | "discussion";
            /** @enum {string} */
            priority: "normal" | "urgent";
            /** @description At most 16384 UTF-8 bytes; never silently truncated */
            body: string;
            /** @enum {string} */
            recipientMode: "participants" | "integrator" | "assignment" | "project";
            recipientIds: string[];
            actionRequired: boolean;
            /** Format: date-time */
            localOccurredAt?: string;
        };
        Message: {
            projectId: string;
            taskId?: string;
            assignmentId?: string;
            threadId: string;
            /** @enum {string} */
            purpose: "question" | "proposal" | "contract-change" | "handoff" | "delivery" | "decision-notice" | "status" | "discussion";
            /** @enum {string} */
            priority: "normal" | "urgent";
            /** @description At most 16384 UTF-8 bytes; never silently truncated */
            body: string;
            /** @enum {string} */
            recipientMode: "participants" | "integrator" | "assignment" | "project";
            recipientIds: string[];
            actionRequired: boolean;
            /** Format: date-time */
            localOccurredAt?: string;
            id: string;
            actorId: string;
            /** @enum {string} */
            actorKind: "human" | "agent";
            sessionId?: string;
            /** Format: date-time */
            receivedAt: string;
            receipts?: {
                participantId: string;
                state: string;
                /** Format: int64 */
                version: number;
                metadata: Record<string, never>;
            }[];
            imported?: boolean;
            /** @description Self-reported original session/time/base versions; not authority */
            offlineSource?: Record<string, never>;
        };
        ReceiptUpdate: {
            /** @enum {string} */
            state: "fetched" | "seen" | "accepted" | "declined" | "resolved";
            note?: string;
            evidenceRefs?: string[];
        };
        Event: {
            /** Format: int64 */
            seq: number;
            scope: string;
            /** @description Typed domain event; a large message uses messageRef and must be fetched separately */
            body: Record<string, never>;
            /** Format: date-time */
            receivedAt: string;
        };
        SyncPage: {
            events: components["schemas"]["Event"][];
            cursor: string;
            hasMore: boolean;
            /** Format: int64 */
            bytes: number;
            /** Format: date-time */
            serverTime: string;
            /** @constant */
            connection: "online";
        };
        PolicyRequirement: {
            id: string;
            text: string;
            /** @enum {string} */
            source: "owner" | "agent" | "system";
        };
        AcceptanceConstraints: {
            /** @enum {string} */
            modeSource: "owner" | "agent" | "system";
            /** @enum {string} */
            acceptorSource: "owner" | "agent" | "system";
            requirements: components["schemas"]["PolicyRequirement"][];
        };
        DecisionOption: {
            id: string;
            label: string;
            impact: string;
            /** @enum {string} */
            outcome?: "answered" | "approved" | "rejected" | "revise";
        };
        /** @description Exactly one Task or intake Request subject. Question identity/history remain unchanged on conversion. Pre-task questions require current preparation version/epoch and block request-convert. Proposals are not authority. */
        DecisionRequest: {
            taskId?: string;
            /** @description system: prefix reserved for service-generated risk questions */
            scopeKey: string;
            question: string;
            reason: string;
            factRefs: string[];
            options: components["schemas"]["DecisionOption"][];
            recommendation: string;
            recommendationReason: string;
            impact: string;
            blockedAssignmentIds: string[];
            blockedActions: ("implementation" | "delivery" | "integration" | "acceptance" | "completion" | "spec-change" | "external-action")[];
            canContinue: string[];
            noDecisionBehavior: string;
            /** Format: date-time */
            deadline?: string;
            /** @enum {string} */
            actionKind?: "decision-pending" | "owner-blocker";
            requestId?: string;
            externalProposal?: components["schemas"]["ExternalAuthorization"];
            specProposal?: components["schemas"]["SpecAmendment"];
        } & (unknown | unknown);
        SpecAmendment: {
            goal: string;
            scope: string[];
            nonScope: string[];
            authorityRefs: string[];
            /** @enum {string} */
            risk: "low" | "normal" | "high";
        };
        ExternalAuthorization: {
            /** @enum {string} */
            action: "push" | "pr" | "merge" | "release" | "deploy";
            target: string;
            scope: string;
            originalText: string;
            /** Format: date-time */
            expiresAt: string;
            /** Format: int64 */
            maxUses: number;
        };
        DecisionResolve: {
            answer: string;
            optionId?: string;
            supersedesDecisionId?: string;
            authorityFollowUpRequired?: boolean;
            externalAuthorization?: components["schemas"]["ExternalAuthorization"];
            /** @enum {string} */
            outcome?: "answered" | "approved" | "rejected" | "revise";
            specAmendment?: components["schemas"]["SpecAmendment"];
        };
        DecisionFollowup: {
            reason: string;
            evidenceRefs: string[];
        };
        ArtifactRef: {
            reference: string;
            digest: string;
        };
        DeliverySubmit: {
            repoId: string;
            baseSha: string;
            headSha: string;
            branchRef: string;
            workspaceRef: string;
            summary: string;
            evidenceRefs: string[];
            unknowns: string[];
            artifactRefs: components["schemas"]["ArtifactRef"][];
        };
        DeliveryReview: {
            deliveryRevisionId: string;
            /** @enum {string} */
            disposition: "accepted" | "returned";
            reason: string;
        };
        IntegrationHead: {
            repoId: string;
            baseSha: string;
            headSha: string;
        };
        EvidenceCheck: {
            condition: string;
            /** @enum {string} */
            outcome: "passed" | "failed" | "unverified";
            headShas: {
                [key: string]: string;
            };
            evidenceRefs: string[];
        };
        IntegrationRecord: {
            deliveryRevisionIds: string[];
            heads: components["schemas"]["IntegrationHead"][];
            checks: components["schemas"]["EvidenceCheck"][];
            summary: string;
            /** @enum {string} */
            result: "passed" | "failed";
            excludedAssignments: {
                assignmentId: string;
                reason: string;
            }[];
            untested: string[];
        };
        AcceptancePolicyChange: {
            policy: components["schemas"]["AcceptancePolicy"];
            reason: string;
        };
        AcceptanceCandidate: {
            integrationId: string;
        };
        AcceptCandidate: {
            candidateId: string;
        };
        Assignment: {
            taskId: string;
            role: string;
            input: string;
            output: string;
            writeScope: string[];
            repoId: string;
            baseRef: string;
            dependencies: string[];
            id: string;
            /** Format: int64 */
            version: number;
            claimParticipantId?: string;
            claimSessionId?: string;
            /** Format: int64 */
            claimEpoch: number;
            /** @enum {string} */
            state: "open" | "claimed" | "active" | "blocked" | "submitted" | "accepted" | "cancelled";
            required: boolean;
            canStart: boolean;
            currentDeliveryId?: string;
            acceptedDeliveryId?: string;
            /** Format: int64 */
            contractVersion: number;
            cancellationReason?: string;
        };
        OriginalCommand: {
            kind: string;
            targetId: string;
            meta: components["schemas"]["OriginalMutation"];
            payload: components["schemas"]["ProjectCreate"] | components["schemas"]["Empty"] | components["schemas"]["TaskCreate"] | components["schemas"]["AssignmentCreate"] | components["schemas"]["Release"] | components["schemas"]["AssignmentUpdate"] | components["schemas"]["Limit"] | components["schemas"]["Reason"] | components["schemas"]["Checkpoint"] | components["schemas"]["Resume"] | components["schemas"]["Handoff"] | components["schemas"]["MessageSend"] | components["schemas"]["ReceiptUpdate"] | components["schemas"]["DecisionRequest"] | components["schemas"]["DecisionResolve"] | components["schemas"]["DecisionFollowup"] | components["schemas"]["DeliverySubmit"] | components["schemas"]["DeliveryReview"] | components["schemas"]["IntegrationRecord"] | components["schemas"]["AcceptancePolicyChange"] | components["schemas"]["AcceptanceCandidate"] | components["schemas"]["AcceptCandidate"];
        };
        OfflineReconcile: {
            projectId: string;
            localId: string;
            originalRequestId: string;
            originalCommand: components["schemas"]["OriginalCommand"];
            baseCursor: string;
            /** Format: date-time */
            localOccurredAt: string;
        };
        OfflineReview: {
            /** @enum {string} */
            disposition: "reissue" | "retain-artifact" | "discard";
            reason: string;
            evidenceRefs: string[];
        };
        OriginalMutation: {
            /** @description May be empty only in an original provisional offline record */
            serviceId: string;
            /** @description May be empty only in an original provisional offline record */
            datasetGeneration: string;
            clientRequestId: string;
            /** Format: int64 */
            expectedVersion?: number;
            /** Format: int64 */
            claimEpoch?: number;
            /** Format: int64 */
            integratorEpoch?: number;
            /** Format: int64 */
            specVersion?: number;
            /** Format: int64 */
            acceptancePolicyVersion?: number;
            sessionId?: string;
            decisionVersions?: {
                [key: string]: number;
            };
        };
        RepositoryView: {
            id: string;
            projectId: string;
            version: number;
            name: string;
            reference: string;
        };
        ProjectView: {
            id: string;
            version: number;
            name: string;
            repositories: components["schemas"]["RepositoryView"][];
            /** @description Returned only to human owners; no enrollment capability. */
            enrollmentEnabled?: boolean;
            /**
             * Format: date-time
             * @description Read-only, derived from the project creation event.
             */
            createdAt?: string;
        };
        ParticipantView: {
            id: string;
            name: string;
        };
        SessionView: {
            id: string;
            participantId: string;
            version: number;
            status: string;
            presence: string;
            /** Format: date-time */
            lastSeenAt: string;
            pendingWaits: number;
        } & {
            [key: string]: unknown;
        };
        /** @description Exactly one Task or intake Request subject. Question identity/history remain unchanged on conversion. Pre-task questions require current preparation version/epoch and block request-convert. Proposals are not authority. */
        DecisionRequestView: {
            taskId?: string;
            /** @description system: prefix reserved for service-generated risk questions */
            scopeKey: string;
            question: string;
            reason: string;
            factRefs: string[];
            options: components["schemas"]["DecisionOption"][];
            recommendation: string;
            recommendationReason: string;
            impact: string;
            blockedAssignmentIds: string[];
            blockedActions: ("implementation" | "delivery" | "integration" | "acceptance" | "completion" | "spec-change" | "external-action")[];
            canContinue: string[];
            noDecisionBehavior: string;
            /** Format: date-time */
            deadline?: string;
            id: string;
            projectId: string;
            version: number;
            state: string;
            actorId: string;
            actorKind: string;
            /** Format: date-time */
            receivedAt: string;
            currentDecisionId?: string;
            decisionVersion: number;
            followUpRequired: boolean;
            followUpComplete: boolean;
            followUpEvidence?: string[];
            specVersion: number;
            approvedSpecVersion: number;
            resolutionOutcome?: string;
            /** @enum {string} */
            actionKind?: "decision-pending" | "owner-blocker";
            requestId?: string;
            externalProposal?: components["schemas"]["ExternalAuthorization"];
            specProposal?: components["schemas"]["SpecAmendment"];
            /**
             * @description Derived at read time for system-authored questions so clients can render them in the reader language; never stored or written.
             * @enum {string}
             */
            messageKey?: "system.riskStart";
        } & (unknown | unknown);
        DecisionView: {
            id: string;
            requestId: string;
            version: number;
            answer: string;
            optionId: string;
            supersedesDecisionId: string;
            actorId: string;
            actorKind: string;
            /** Format: date-time */
            receivedAt: string;
            authorityFollowUpRequired: boolean;
            outcome: string;
            specVersion: number;
            externalAuthorization: components["schemas"]["ExternalAuthorization"] | null;
            specAmendment: components["schemas"]["SpecAmendment"] | null;
        };
        DecisionHistoryView: {
            request: components["schemas"]["DecisionRequestView"];
            history: components["schemas"]["DecisionView"][];
        };
        DeliveryView: {
            repoId: string;
            baseSha: string;
            headSha: string;
            branchRef: string;
            workspaceRef: string;
            summary: string;
            evidenceRefs: string[];
            unknowns: string[];
            artifactRefs: components["schemas"]["ArtifactRef"][];
            id: string;
            taskId: string;
            assignmentId: string;
            revision: number;
            actorId: string;
            actorKind: string;
            sessionId: string;
            claimEpoch: number;
            specVersion: number;
            decisionVersions: {
                [key: string]: number;
            };
            /** Format: date-time */
            receivedAt: string;
            assignmentContractVersion: number;
        };
        IntegrationView: {
            deliveryRevisionIds: string[];
            heads: components["schemas"]["IntegrationHead"][];
            checks: components["schemas"]["EvidenceCheck"][];
            summary: string;
            /** @enum {string} */
            result: "passed" | "failed";
            excludedAssignments: {
                assignmentId: string;
                reason: string;
            }[];
            untested: string[];
            id: string;
            taskId: string;
            specVersion: number;
            acceptancePolicyVersion: number;
            decisionVersions: {
                [key: string]: number;
            };
            integratorEpoch: number;
            actorId: string;
            actorKind: string;
            sessionId: string;
            /** Format: date-time */
            receivedAt: string;
        };
        CandidateView: {
            id: string;
            taskId: string;
            integration: components["schemas"]["IntegrationView"];
            policy: components["schemas"]["AcceptancePolicy"];
            constraints: components["schemas"]["AcceptanceConstraints"];
            actorId: string;
            /** Format: date-time */
            receivedAt: string;
        };
        CandidateStatusView: {
            candidate: components["schemas"]["CandidateView"];
            stale: boolean;
            evidenceSource: string;
        };
        AcceptanceView: {
            id: string;
            taskId: string;
            candidateId: string;
            actorId: string;
            participantId: string;
            actorKind: string;
            sessionId: string;
            integratorEpoch: number;
            acceptancePolicyVersion: number;
            /** Format: date-time */
            receivedAt: string;
            source: string;
        };
        CompletionView: {
            id: string;
            taskId: string;
            taskVersion: number;
            candidate: components["schemas"]["CandidateView"];
            acceptanceId: string;
            actorId: string;
            actorKind: string;
            /** Format: date-time */
            receivedAt: string;
            evidenceSource: string;
        };
        BindingPayload: components["schemas"]["GitHubBindingPayload"] | components["schemas"]["NotificationBindingPayload"];
        GitHubBindingPayload: {
            projectId: string;
            /** @enum {string} */
            kind: "github";
            repository: string;
            repositoryId: number;
            private: boolean;
            fields: ("taskId" | "goal" | "state" | "scope" | "acceptance")[];
            events: ("task.create" | "task.complete" | "task.cancel" | "task.reopen" | "integration.record" | "decision.request" | "decision.resolve" | "decision.supersede" | "delivery.submit" | "delivery.review" | "acceptance.candidate" | "acceptance.accept")[];
            credentialRef: string;
            enabled: boolean;
        };
        NotificationBindingPayload: {
            projectId: string;
            /** @enum {string} */
            kind: "dingtalk";
            fields: ("taskId" | "goal" | "summary" | "projectTitle")[];
            events: ("decision-pending" | "human-acceptance" | "owner-blocker")[];
            credentialRef: string;
            enabled: boolean;
        };
        ControlIdentity: {
            serviceId: string;
            datasetGeneration: string;
        };
        ControlProof: {
            /** @constant */
            protocolVersion: 1;
            serviceId: string;
            datasetGeneration: string;
            dataRoot: string;
            executable: string;
            needsActivation: boolean;
            /**
             * @description Presentation-only owner language for the native tray; omitted when unset. Older launchers ignore it.
             * @enum {string}
             */
            ownerLanguage?: "en" | "zh-CN" | "zh-TW" | "ja";
        };
        /** @description Human-only durable intent, not an executable Task. Optional owner requirements must be preserved during conversion. */
        RequestCreate: {
            projectId: string;
            text: string;
            constraints?: string[];
            collaboratorLimit?: number;
            designatedIntegratorId?: string;
            acceptancePolicy?: components["schemas"]["AcceptancePolicy"];
        };
        /** @description Human-only durable intent, not an executable Task. Optional owner requirements must be preserved during conversion. */
        WorkRequest: {
            projectId: string;
            text: string;
            constraints?: string[];
            collaboratorLimit?: number;
            designatedIntegratorId?: string;
            acceptancePolicy?: components["schemas"]["AcceptancePolicy"];
            id: string;
            version: number;
            createdBy: string;
            actorKind: string;
            /** Format: date-time */
            createdAt: string;
            /** @enum {string} */
            state: "waiting" | "preparing" | "converted" | "cancelled";
            preparerParticipantId?: string;
            preparerSessionId?: string;
            preparationEpoch: number;
            offeredSessionId?: string;
            taskId?: string;
            convertedSpecVersion?: number;
            lastReason?: string;
        };
        RequestHandoff: {
            reason: string;
            toSessionId: string;
        };
        RequestResume: {
            expectedSourceSessionId: string;
        };
        /** @description Owner-only forced preparation takeover; expectedVersion must match the displayed Request. No timeout stealing. */
        RequestAssign: {
            reason: string;
            toSessionId: string;
            expectedSourceSessionId?: string;
        };
        /** @description Current preparer only, Request expectedVersion and preparationEpoch in claimEpoch. Requires acknowledged formal decision versions. Atomically returns the linked WorkRequest; its taskId names the unique complete Task. */
        RequestConvert: {
            task: components["schemas"]["TaskCreate"];
        };
        /** @description Human cookie + Origin + CSRF only. Secret material stored only in a product-private file; receipt contains credentialRef and kind. */
        BindingCredential: ({
            /** @enum {string} */
            kind: "github" | "dingtalk";
            token?: string;
            secret?: string;
            /** @description Exact official HTTPS DingTalk URL with a single access_token. No arbitrary endpoint. */
            webhookUrl?: string;
        } & unknown) & (unknown | unknown);
        /** @description Owner-only. Repository URL or owner/repo; GET official api.github.com only, no redirects or probe write. Metadata read cannot establish Issues write permission. */
        BindingInspectionInput: {
            repository: string;
            credentialRef: string;
        };
        RepositoryInspection: {
            repository: string;
            repositoryId: number;
            private: boolean;
            hasIssues: boolean;
            /** @constant */
            issuesWriteVerified: false;
        };
        ConnectionInfo: {
            clientPath: string;
            skillPath: string;
            dataDir: string;
            serviceURL: string;
        };
        /** @description Human cookie + Origin + CSRF only. Sets the owner language used by the browser, activation page, tray and notifications; an empty value clears it. Grants no authority and is not readable by agents. */
        OwnerLanguage: {
            /** @enum {string} */
            language: "" | "en" | "zh-CN" | "zh-TW" | "ja";
        };
        /** @description Owner-only room read: one item only the owner can move forward. Aggregates existing records; grants nothing. */
        NeedsYouItem: {
            /** @enum {string} */
            kind: "decision" | "acceptance" | "blocker";
            projectId: string;
            taskId?: string;
            requestId?: string;
            decisionRequestId?: string;
            candidateId?: string;
            /** @description Blocker code: integrator-revoked, integrator-session-ended, pending-reconciliation:<id>, preparer-session-ended or preparer-revoked. */
            code?: string;
            /**
             * @description Present only for system-generated questions; the UI renders the localized template and falls back to summary.
             * @enum {string}
             */
            messageKey?: "system.riskStart";
            summary: string;
            /** Format: date-time */
            since: string;
        };
        /** @description Owner-only room feed entry, newest first; message bodies come from the thread query. */
        ActivityEntry: {
            /** Format: int64 */
            seq: number;
            kind: string;
            resourceId: string;
            taskId?: string;
            requestId?: string;
            actorId?: string;
            actorKind?: string;
            /** Format: date-time */
            receivedAt: string;
            imported: boolean;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    activateOwner: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/x-www-form-urlencoded": {
                    capability: string;
                };
            };
        };
        responses: {
            /** @description Owner HttpOnly SameSite Strict cookie; redirect to setup */
            303: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Expired or consumed capability */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    currentActor: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Authenticated actor; CSRF, ownerLanguage and build information returned only to the same-origin owner cookie session */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
        };
    };
    enrollAgent: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Enrollment"];
            };
        };
        responses: {
            /** @description Credentials in result; caller stores privately */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
            /** @description Structured error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
        };
    };
    applyCommand: {
        parameters: {
            query?: never;
            header?: {
                "X-CSRF-Token"?: string;
            };
            path: {
                kind: "project.create" | "enrollment.enable" | "participant.revoke" | "task.create" | "integrator.claim" | "assignment.create" | "assignment.claim" | "assignment.release" | "assignment.update" | "collaborator.limit" | "task.reopen" | "task.cancel" | "session.checkpoint" | "session.end" | "session.resume" | "integrator.handoff" | "message.send" | "receipt.update" | "decision.request" | "decision.resolve" | "decision.supersede" | "decision.withdraw" | "decision.followup" | "delivery.submit" | "delivery.review" | "integration.record" | "acceptance.policy" | "acceptance.candidate" | "acceptance.accept" | "task.complete" | "offline.reconcile" | "offline.resolve" | "backup.create" | "backup.export" | "participant.recover" | "binding.create" | "binding.disable" | "binding.retry" | "binding.credential" | "binding.rotate" | "request.create" | "request.claim" | "request.release" | "request.handoff" | "request.accept" | "request.resume" | "request.assign" | "request.cancel" | "request.convert" | "binding.inspect" | "owner.language";
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Mutation"];
            };
        };
        responses: {
            /** @description Atomic committed receipt */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
            /** @description Structured error; 400 invalid, 401 unauthenticated, 403 forbidden, 409 stale, 413 body budget */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
        };
    };
    health: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Minimal service identity; no tasks or credentials */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Identity"];
                };
            };
        };
    };
    readWorkspace: {
        parameters: {
            query?: {
                projectId?: string;
                targetId?: string;
                cursor?: string;
                limit?: number;
            };
            header?: never;
            path: {
                kind: "projects" | "tasks" | "task" | "assignments" | "assignment" | "sessions" | "attention" | "inbox" | "thread" | "message" | "decisions" | "decision" | "deliveries" | "delivery" | "integrations" | "candidate" | "acceptances" | "completions" | "offline-candidates" | "offline-candidate" | "metrics" | "participants" | "candidates" | "bindings" | "outbound-jobs" | "requests" | "request" | "needs-you" | "activity";
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Scoped read snapshot */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
            /** @description Structured error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
        };
    };
    createWorkSession: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": {
                    serviceId: string;
                    datasetGeneration: string;
                    clientRequestId: string;
                };
            };
        };
        responses: {
            /** @description Credentials in result; store privately */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
            /** @description Structured error */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
        };
    };
    identity: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Identity to compare before transmitting client credentials */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Identity"];
                };
            };
        };
    };
    syncWorkspace: {
        parameters: {
            query?: {
                cursor?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Durable events; no automatic fetched/seen receipt */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
            /** @description Structured error including cursor_reset */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
        };
    };
    waitWorkspace: {
        parameters: {
            query?: {
                cursor?: string;
                timeoutSeconds?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Durable events; no automatic fetched/seen receipt */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
            /** @description Structured error including cursor_reset */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
        };
    };
    eventsWorkspace: {
        parameters: {
            query?: {
                cursor?: string;
            };
            header?: {
                "Last-Event-ID"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Durable events; no automatic fetched/seen receipt */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "text/event-stream": string;
                };
            };
            /** @description Structured error including cursor_reset */
            default: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Error"];
                };
            };
        };
    };
    ownServiceProbe: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ControlIdentity"];
            };
        };
        responses: {
            /** @description Own product control result; no capability in the response */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ControlProof"];
                };
            };
            /** @description Agent/owner-cookie/foreign-origin requests cannot use this capability */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Service identity changed */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    ownServiceActivate: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ControlIdentity"];
            };
        };
        responses: {
            /** @description Own product control result; no capability in the response */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        activationPath?: string;
                    };
                };
            };
            /** @description Agent/owner-cookie/foreign-origin requests cannot use this capability */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Service identity changed */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    ownServiceStop: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ControlIdentity"];
            };
        };
        responses: {
            /** @description Own product control result; no capability in the response */
            202: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        stopping?: boolean;
                    };
                };
            };
            /** @description Agent/owner-cookie/foreign-origin requests cannot use this capability */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Service identity changed */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    ownerConnectionInstructions: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Owner connection metadata */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Envelope"];
                };
            };
            /** @description Agents forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
}
