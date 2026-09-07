package dify

import (
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
)

// MapResult is the fan-out of one Dify application.
type MapResult struct {
	Candidates []domain.CapabilityCandidate
	Warnings   []string
}

// Map turns a parsed DSL into capability candidates.
//
// A single Dify app is not one capability: it is an agent, the workflow that drives it,
// every tool and knowledge base it binds, the prompt behind each llm node, and the
// business skill that llm node implements. Splitting them is what makes the assets
// reusable in a different composition later.
//
//	app (chat modes)      -> agent            (extracted)
//	workflow.graph        -> workflow         (extracted)
//	tool node             -> tool             (extracted)
//	knowledge-retrieval   -> knowledge_data   (extracted)
//	llm node prompt       -> prompt_template  (extracted)
//	llm node function     -> skill            (inferred)
func Map(dsl DSL, sourceID, sourceSystem string) MapResult {
	graph := dsl.Graph()
	result := MapResult{}

	newCandidate := func(capType domain.CapabilityType, externalID, name, description string,
		status domain.CandidateStatus, raw map[string]any, fields domain.CapabilityFields) domain.CapabilityCandidate {
		fields.Name = name
		fields.Description = description
		return domain.CapabilityCandidate{
			SourceID:      sourceID,
			SourceSystem:  sourceSystem,
			ExternalID:    externalID,
			CandidateType: capType,
			Name:          name,
			Description:   description,
			RawPayload:    raw,
			Extracted:     fields,
			Status:        status,
		}
	}

	// Tools, knowledge and prompts first: the agent, workflow and skill candidates all
	// declare dependencies on them.
	var toolRefs, knowledgeRefs, promptRefs, skillRefs []domain.DependencyRef

	for _, node := range graph.NodesOfType(NodeTool) {
		externalID := firstNonEmpty(node.Data.ProviderID, node.Data.ToolName, node.ID)
		name := firstNonEmpty(node.Data.Title, externalID)
		result.Candidates = append(result.Candidates, newCandidate(
			domain.CapabilityTypeTool, externalID, name, node.Data.Desc,
			domain.CandidateStatusExtracted,
			map[string]any{
				"node_id":       node.ID,
				"provider_id":   node.Data.ProviderID,
				"provider_name": node.Data.ProviderName,
				"provider_type": node.Data.ProviderType,
				"tool_name":     node.Data.ToolName,
			},
			domain.CapabilityFields{
				Subtype:      firstNonEmpty(node.Data.ProviderType, "builtin"),
				InputSchema:  schemaFromToolParams(node.Data.ToolParams),
				OutputSchema: objectSchema(map[string]string{"result": "object"}, nil),
			}))
		toolRefs = append(toolRefs, domain.DependencyRef{
			ExternalID: externalID, TargetType: domain.CapabilityTypeTool,
			DependencyType: domain.DependencyUsesTool,
		})
	}

	for _, node := range graph.NodesOfType(NodeKnowledgeRetrieval) {
		externalID := node.ID
		if len(node.Data.DatasetIDs) > 0 && node.Data.DatasetIDs[0] != "" {
			externalID = node.Data.DatasetIDs[0]
		}
		name := firstNonEmpty(node.Data.Title, externalID)
		result.Candidates = append(result.Candidates, newCandidate(
			domain.CapabilityTypeKnowledgeData, externalID, name, node.Data.Desc,
			domain.CandidateStatusExtracted,
			map[string]any{"node_id": node.ID, "dataset_ids": node.Data.DatasetIDs},
			domain.CapabilityFields{
				Subtype:      "dataset",
				InputSchema:  objectSchema(map[string]string{"query": "string"}, []string{"query"}),
				OutputSchema: objectSchema(map[string]string{"documents": "array"}, nil),
			}))
		knowledgeRefs = append(knowledgeRefs, domain.DependencyRef{
			ExternalID: externalID, TargetType: domain.CapabilityTypeKnowledgeData,
			DependencyType: domain.DependencyUsesKnowledge,
		})
	}

	for _, node := range graph.NodesOfType(NodeLLM) {
		title := firstNonEmpty(node.Data.Title, node.ID)
		prompt := node.SystemPrompt()

		promptName := title + " Prompt"
		promptID := domain.Slugify(promptName)
		result.Candidates = append(result.Candidates, newCandidate(
			domain.CapabilityTypePromptTemplate, promptID, promptName,
			describePrompt(title, node.Data.Desc),
			domain.CandidateStatusExtracted,
			map[string]any{"node_id": node.ID, "prompt_template": prompt, "model": node.Data.Model},
			domain.CapabilityFields{
				Subtype:  "system_prompt",
				Metadata: map[string]any{"prompt_text": prompt},
			}))
		promptRefs = append(promptRefs, domain.DependencyRef{
			ExternalID: promptID, TargetType: domain.CapabilityTypePromptTemplate,
			DependencyType: domain.DependencyUsesPrompt,
		})

		// The business function an llm node performs is not stated anywhere in the DSL,
		// so the skill is inferred rather than extracted. It is the reusable unit: the
		// prompt is its implementation, the tools and datasets are its bindings.
		skillName := title + " Skill"
		// The skill is identified by the business function itself, not by the word
		// "skill": `product-recommendation`, not `product-recommendation-skill`. That is
		// the name a composition references it by.
		skillID := domain.Slugify(title)
		skillDeps := append(append(append([]domain.DependencyRef{}, toolRefs...), knowledgeRefs...),
			domain.DependencyRef{
				ExternalID: promptID, TargetType: domain.CapabilityTypePromptTemplate,
				DependencyType: domain.DependencyUsesPrompt,
			})
		result.Candidates = append(result.Candidates, newCandidate(
			domain.CapabilityTypeSkill, skillID, skillName,
			describeSkill(title, node.Data.Desc),
			domain.CandidateStatusInferred,
			map[string]any{"node_id": node.ID, "inferred_from": "llm node"},
			domain.CapabilityFields{
				Subtype:      "llm_skill",
				InputSchema:  schemaFromVariables(startVariables(graph)),
				OutputSchema: objectSchema(map[string]string{"text": "string"}, nil),
				DependsOn:    skillDeps,
				Metadata:     map[string]any{"inferred_from_node": node.ID},
			}))
		skillRefs = append(skillRefs, domain.DependencyRef{
			ExternalID: skillID, TargetType: domain.CapabilityTypeSkill,
			DependencyType: domain.DependencyCallsSkill,
		})
	}

	bindings := append(append(append([]domain.DependencyRef{}, toolRefs...), knowledgeRefs...), promptRefs...)
	bindings = append(bindings, skillRefs...)

	// The workflow is the executable graph.
	var workflowRef []domain.DependencyRef
	if len(graph.Nodes) >= 2 {
		workflowName := workflowNameFor(dsl.App.Name)
		workflowID := domain.Slugify(workflowName)
		steps := graph.OrderedSteps()
		result.Candidates = append(result.Candidates, newCandidate(
			domain.CapabilityTypeWorkflow, workflowID, workflowName,
			describeWorkflow(dsl.App, steps),
			domain.CandidateStatusExtracted,
			map[string]any{"app_mode": dsl.App.Mode, "node_count": len(graph.Nodes), "steps": steps},
			domain.CapabilityFields{
				Subtype:      "dify_workflow",
				InputSchema:  schemaFromVariables(startVariables(graph)),
				OutputSchema: objectSchema(map[string]string{"answer": "string"}, nil),
				DependsOn:    bindings,
				Metadata:     map[string]any{"steps": steps},
			}))
		workflowRef = []domain.DependencyRef{{
			ExternalID: workflowID, TargetType: domain.CapabilityTypeWorkflow,
			DependencyType: domain.DependencyContainsStep,
		}}
	} else {
		result.Warnings = append(result.Warnings,
			"graph has fewer than two nodes; no workflow capability was extracted")
	}

	// Only conversational Dify modes describe an agent; a bare `workflow` app does not.
	if dsl.IsAgentMode() {
		agentID := domain.Slugify(dsl.App.Name)
		result.Candidates = append(result.Candidates, newCandidate(
			domain.CapabilityTypeAgent, agentID, dsl.App.Name, dsl.App.Description,
			domain.CandidateStatusExtracted,
			map[string]any{"app_mode": dsl.App.Mode, "kind": dsl.Kind, "version": dsl.Version},
			domain.CapabilityFields{
				Subtype:      dsl.App.Mode,
				InputSchema:  schemaFromVariables(startVariables(graph)),
				OutputSchema: objectSchema(map[string]string{"answer": "string"}, nil),
				DependsOn:    append(append([]domain.DependencyRef{}, workflowRef...), bindings...),
			}))
	} else {
		result.Warnings = append(result.Warnings,
			"app mode "+firstNonEmpty(dsl.App.Mode, "(unset)")+" is not conversational; no agent capability was extracted")
	}

	return result
}

// workflowNameFor derives the workflow's display name from the app name without
// producing "... Workflow Workflow" for apps that are already named as workflows.
func workflowNameFor(appName string) string {
	trimmed := strings.TrimSpace(appName)
	if strings.HasSuffix(strings.ToLower(trimmed), "workflow") {
		return trimmed
	}
	trimmed = strings.TrimSuffix(trimmed, " Agent")
	return trimmed + " Workflow"
}

func describeWorkflow(app App, steps []string) string {
	base := "Workflow extracted from the Dify application " + app.Name + "."
	if len(steps) > 0 {
		base += " Steps: " + strings.Join(steps, " -> ") + "."
	}
	return base
}

func describePrompt(title, desc string) string {
	if strings.TrimSpace(desc) != "" {
		return desc
	}
	return "Prompt template behind the " + title + " step."
}

func describeSkill(title, desc string) string {
	if strings.TrimSpace(desc) != "" {
		return desc
	}
	return "Business skill implemented by the " + title + " step."
}

func startVariables(g Graph) []Variable {
	for _, node := range g.NodesOfType(NodeStart) {
		if len(node.Data.Variables) > 0 {
			return node.Data.Variables
		}
	}
	return nil
}

func schemaFromVariables(vars []Variable) map[string]any {
	properties := map[string]any{}
	var required []string
	for _, v := range vars {
		name := firstNonEmpty(v.Variable, v.Label)
		if name == "" {
			continue
		}
		properties[name] = map[string]any{
			"type":        jsonType(v.Type),
			"description": v.Label,
		}
		if v.Required {
			required = append(required, name)
		}
	}
	return schemaDocument(properties, required)
}

func schemaFromToolParams(params map[string]any) map[string]any {
	properties := map[string]any{}
	for name, value := range params {
		properties[name] = map[string]any{"type": goValueType(value)}
	}
	return schemaDocument(properties, nil)
}

func objectSchema(fields map[string]string, required []string) map[string]any {
	properties := map[string]any{}
	for name, t := range fields {
		properties[name] = map[string]any{"type": t}
	}
	return schemaDocument(properties, required)
}

func schemaDocument(properties map[string]any, required []string) map[string]any {
	doc := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		doc["required"] = required
	}
	return doc
}

func jsonType(difyType string) string {
	switch strings.ToLower(difyType) {
	case "number", "float", "integer", "int":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "array", "select":
		return "array"
	case "object", "json":
		return "object"
	default:
		return "string"
	}
}

func goValueType(value any) string {
	switch value.(type) {
	case bool:
		return "boolean"
	case int, int64, float64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "string"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
