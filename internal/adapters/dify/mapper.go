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

	// Tool-like nodes. A Dify tool carries a real provider id; the built-in HTTP request
	// and document extractor nodes do not, so they are identified by the app and node
	// title instead. All three are externally provisioned work, so all three are tools.
	toolLikeNodes := append(append(
		append([]Node{}, graph.NodesOfType(NodeTool)...),
		graph.NodesOfType(NodeHTTPRequest)...),
		graph.NodesOfType(NodeDocumentExtractor)...)

	for _, node := range toolLikeNodes {
		title := qualifiedNodeTitle(dsl.App.Name, node)
		externalID := firstNonEmpty(node.Data.ProviderID, node.Data.ToolName, domain.Slugify(title))
		name := firstNonEmpty(node.Data.Title, externalID)
		if isDefaultTitle(node.Data.Title) {
			name = title
		}
		result.Candidates = append(result.Candidates, newCandidate(
			domain.CapabilityTypeTool, externalID, name, node.Data.Desc,
			domain.CandidateStatusExtracted,
			map[string]any{
				"node_id":       node.ID,
				"node_type":     node.Kind(),
				"provider_id":   node.Data.ProviderID,
				"provider_name": node.Data.ProviderName,
				"provider_type": node.Data.ProviderType,
				"tool_name":     node.Data.ToolName,
			},
			domain.CapabilityFields{
				Subtype:      firstNonEmpty(node.Data.ProviderType, node.Kind()),
				InputSchema:  schemaFromToolParams(node.Data.ToolParams),
				OutputSchema: objectSchema(map[string]string{"result": "object"}, nil),
			}))
		toolRefs = append(toolRefs, domain.DependencyRef{
			ExternalID: externalID, TargetType: domain.CapabilityTypeTool,
			DependencyType: domain.DependencyUsesTool,
		})
	}

	for _, node := range graph.NodesOfType(NodeKnowledgeRetrieval) {
		title := qualifiedNodeTitle(dsl.App.Name, node)
		externalID := domain.Slugify(title)
		if len(node.Data.DatasetIDs) > 0 && node.Data.DatasetIDs[0] != "" {
			externalID = node.Data.DatasetIDs[0]
		}
		name := firstNonEmpty(node.Data.Title, externalID)
		if isDefaultTitle(node.Data.Title) {
			name = title
		}
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
		// A node left on Dify's default title ("LLM") says nothing about what it does and
		// would collide with every other default-titled node in the tenant, so it is
		// qualified by the application it belongs to.
		title := qualifiedNodeTitle(dsl.App.Name, node)
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
			"this app is a "+firstNonEmpty(dsl.App.Mode, "(unset)")+
				", not a conversational agent, so its capabilities were extracted without an agent")
	}

	return result
}

// defaultNodeTitles are the labels Dify gives a node when the author never renames it,
// in the locales the console ships. Such a title identifies the node's *mechanism*, not
// its business function, so on its own it is not a usable capability name.
var defaultNodeTitles = map[string]bool{
	"llm":                 true,
	"tool":                true,
	"knowledge retrieval": true,
	"http request":        true,
	"doc extractor":       true,
	"document extractor":  true,
	"agent":               true,
	"answer":              true,
	"start":               true,
	"end":                 true,
	"code":                true,
	// Simplified Chinese console defaults.
	"大模型":   true,
	"知识检索":  true,
	"工具":    true,
	"直接回复":  true,
	"开始":    true,
	"结束":    true,
	"文档提取器": true,
}

// isDefaultTitle reports whether a node title was left at Dify's default.
func isDefaultTitle(title string) bool {
	return defaultNodeTitles[strings.ToLower(strings.TrimSpace(title))]
}

// qualifiedNodeTitle names a node for capability purposes.
//
// A renamed node is already meaningful on its own ("Product Recommendation"), and using
// it unqualified keeps composition YAML readable. A node still on its default title is
// qualified by its application, because "LLM" would otherwise be the identity of every
// untitled llm node in the tenant — silently merging unrelated capabilities.
func qualifiedNodeTitle(appName string, node Node) string {
	title := strings.TrimSpace(node.Data.Title)
	if title != "" && !isDefaultTitle(title) {
		return title
	}
	app := strings.TrimSpace(appName)
	switch {
	case app != "" && title != "":
		return app + " " + title
	case app != "":
		return app + " " + node.ID
	case title != "":
		return title
	default:
		return node.ID
	}
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
	for _, node := range g.NodesOfType("start") {
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
