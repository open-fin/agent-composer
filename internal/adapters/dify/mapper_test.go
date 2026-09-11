package dify_test

import (
	"strings"
	"testing"

	"github.com/open-fin/agent-composer/internal/adapters/dify"
	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
)

// realWorldDSL mirrors the shape of a DSL exported from a live Dify console: a
// Chinese application name, nodes left on their default titles, a document-extractor
// node, and canvas notes that carry no data.type.
const realWorldDSL = `
app:
  name: AI 摘要助手
  mode: workflow
  description: 上传文档并生成摘要
kind: app
version: 0.1.5
workflow:
  graph:
    nodes:
      - id: "1776673700000"
        type: custom
        data:
          type: start
          title: 开始
          variables:
            - variable: file
              label: 文档
              type: file
              required: true
      - id: "1776673793380"
        type: custom
        data:
          type: document-extractor
          title: 文档提取器
      - id: "1776673800001"
        type: custom
        data:
          type: llm
          title: LLM
          prompt_template:
            - role: system
              text: 请总结下面的文档内容。
      - id: "1776673804765"
        type: custom-note
        data:
          title: 备注
          text: 记得检查 token 上限
      - id: "1776673843040"
        type: custom-note
        data:
          text: TODO
      - id: "1776673850896"
        data: {}
      - id: "1776673900000"
        type: custom
        data:
          type: end
          title: 结束
    edges:
      - {id: e1, source: "1776673700000", target: "1776673793380"}
      - {id: e2, source: "1776673793380", target: "1776673800001"}
      - {id: e3, source: "1776673800001", target: "1776673900000"}
`

// secondAppDSL is a different application whose llm node is also left on the default
// title. Before qualification, both apps produced the identifier "llm".
const secondAppDSL = `
app:
  name: 智能客服助手
  mode: workflow
  description: 回答客户常见问题
kind: app
version: 0.1.5
workflow:
  graph:
    nodes:
      - id: "n1"
        data:
          type: start
          title: 开始
      - id: "n2"
        data:
          type: llm
          title: LLM
          prompt_template:
            - role: system
              text: 请回答客户问题。
    edges:
      - {id: e1, source: n1, target: n2}
`

func mapDSL(t *testing.T, payload string) dify.MapResult {
	t.Helper()
	parsed, err := dify.Parse([]byte(payload))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result := dify.Map(parsed.DSL, "src-1", "dify")
	result.Warnings = append(parsed.Warnings, result.Warnings...)
	return result
}

func slugOf(t *testing.T, candidate domain.CapabilityCandidate) string {
	t.Helper()
	return engine.SlugFor(candidate.ExternalID, candidate.Name)
}

// TestChineseNamesProduceDistinctSlugs is the regression for the bug that made every
// pure-CJK name collapse onto a single identifier, silently merging unrelated
// capabilities under one registry entry.
func TestChineseNamesProduceDistinctSlugs(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"AI 摘要助手", "ai-摘要助手"},
		{"智能客服助手", "智能客服助手"},
		{"合规检查助手", "合规检查助手"},
	} {
		if got := domain.Slugify(tc.name); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
	if domain.Slugify("智能客服助手") == domain.Slugify("合规检查助手") {
		t.Fatal("two different Chinese names still collapse onto the same slug")
	}
}

// TestDocumentExtractorBecomesATool covers the node type that was previously reported
// as unsupported and dropped.
func TestDocumentExtractorBecomesATool(t *testing.T) {
	result := mapDSL(t, realWorldDSL)

	var found bool
	for _, candidate := range result.Candidates {
		if candidate.CandidateType != domain.CapabilityTypeTool {
			continue
		}
		if candidate.RawPayload["node_type"] == "document-extractor" {
			found = true
			if candidate.ExternalID == "1776673793380" {
				t.Error("tool identity should not be the numeric node id")
			}
		}
	}
	if !found {
		t.Fatalf("document-extractor produced no tool candidate; got %v", candidateSummary(result))
	}
}

// TestCanvasNotesAreSilent checks that sticky notes and empty nodes no longer generate
// warnings that look like failures.
func TestCanvasNotesAreSilent(t *testing.T) {
	result := mapDSL(t, realWorldDSL)
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "no data.type") {
			t.Errorf("canvas notes should not warn: %q", warning)
		}
		if strings.Contains(warning, "document-extractor") {
			t.Errorf("document-extractor is supported now: %q", warning)
		}
	}
}

// TestDefaultTitledNodesDoNotCollideAcrossApps is the regression for unrelated
// applications sharing an identifier because both left a node titled "LLM".
func TestDefaultTitledNodesDoNotCollideAcrossApps(t *testing.T) {
	first := mapDSL(t, realWorldDSL)
	second := mapDSL(t, secondAppDSL)

	slugsOf := func(result dify.MapResult, capType domain.CapabilityType) []string {
		var out []string
		for _, candidate := range result.Candidates {
			if candidate.CandidateType == capType {
				out = append(out, slugOf(t, candidate))
			}
		}
		return out
	}

	for _, capType := range []domain.CapabilityType{
		domain.CapabilityTypeSkill, domain.CapabilityTypePromptTemplate,
	} {
		a, b := slugsOf(first, capType), slugsOf(second, capType)
		if len(a) == 0 || len(b) == 0 {
			t.Fatalf("expected %s candidates from both apps, got %v and %v", capType, a, b)
		}
		for _, left := range a {
			for _, right := range b {
				if left == right {
					t.Errorf("%s slug %q is shared by two unrelated applications", capType, left)
				}
			}
		}
		if a[0] == "llm" || b[0] == "llm" {
			t.Errorf("a default node title leaked through as the identity: %v %v", a, b)
		}
	}
}

// TestRenamedNodeKeepsItsOwnName confirms qualification only applies to default titles,
// so a deliberately named node still reads cleanly in composition YAML.
func TestRenamedNodeKeepsItsOwnName(t *testing.T) {
	const named = `
app:
  name: Product Recommendation Workflow
  mode: workflow
workflow:
  graph:
    nodes:
      - id: start
        data: {type: start, title: Start}
      - id: rec
        data:
          type: llm
          title: Product Recommendation
          prompt_template:
            - role: system
              text: Rank products.
    edges:
      - {id: e1, source: start, target: rec}
`
	result := mapDSL(t, named)
	for _, candidate := range result.Candidates {
		if candidate.CandidateType == domain.CapabilityTypeSkill {
			if got := slugOf(t, candidate); got != "product-recommendation" {
				t.Errorf("skill slug = %q, want product-recommendation", got)
			}
			return
		}
	}
	t.Fatal("no skill candidate produced")
}

// TestNumericNodeIDsNeverBecomeSlugs guards against Dify's generated node ids leaking
// into composition YAML as capability identities.
func TestNumericNodeIDsNeverBecomeSlugs(t *testing.T) {
	result := mapDSL(t, realWorldDSL)
	for _, candidate := range result.Candidates {
		slug := slugOf(t, candidate)
		if strings.HasPrefix(slug, "17766737") {
			t.Errorf("%s kept a numeric node id as its slug: %q", candidate.CandidateType, slug)
		}
		if slug == "capability" {
			t.Errorf("%s degenerated to the fallback slug", candidate.CandidateType)
		}
	}
}

func candidateSummary(result dify.MapResult) []string {
	out := make([]string, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		out = append(out, string(candidate.CandidateType)+":"+candidate.ExternalID)
	}
	return out
}
