package llm

import (
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
)

const enrichCandidateSystemPrompt = `You classify enterprise AI capabilities for a capability registry.
Return ONLY a JSON object with these optional keys:
  business_domain (string), intents (string[]), tags (string[]),
  risk_level ("low"|"medium"|"high"), description (string), subtype (string),
  permissions (string[]).
Use snake_case for intents and permissions. Be conservative: omit a key rather than guess.`

const enrichCandidateUserPrompt = `Classify this capability candidate:

%s`

const decomposeGoalSystemPrompt = `You decompose a business goal into the ordered set of capabilities an agent needs.
Return ONLY a JSON object of the form:
  {"roles": [{"step": "snake_case_step", "capability_type": "<type>",
              "description": "...", "keywords": ["..."], "intents": ["..."]}]}
List every step the goal implies, including steps no existing system may cover.
Order the steps in the sequence they would execute.`

const decomposeGoalUserPrompt = `Business goal:
%s

Allowed capability_type values: %s`

func capabilityTypeList() string {
	names := make([]string, 0, len(domain.AllCapabilityTypes))
	for _, t := range domain.AllCapabilityTypes {
		names = append(names, string(t))
	}
	return strings.Join(names, ", ")
}
