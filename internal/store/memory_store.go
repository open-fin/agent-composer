package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/open-fin/agent-composer/internal/domain"
)

// Memory is a map-backed Store. It is the default when no DSN is configured, and is
// what the engine tests and `agentctl` run against, so the entire demo flow is
// exercisable without Postgres.
type Memory struct {
	mu sync.RWMutex

	sources      map[string]domain.ExternalSource
	candidates   map[string]domain.CapabilityCandidate
	capabilities map[string]domain.Capability
	dependencies map[string][]domain.Dependency
	drafts       map[string]domain.BusinessAgentDraft
	components   map[string][]domain.CompositionComponent

	// seq gives every write a monotonic ordering key so listings are deterministic
	// even when several records are created inside the same clock tick.
	seq    int64
	seqOf  map[string]int64
	nowFn  func() time.Time
	closed bool
}

// NewMemory builds an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		sources:      map[string]domain.ExternalSource{},
		candidates:   map[string]domain.CapabilityCandidate{},
		capabilities: map[string]domain.Capability{},
		dependencies: map[string][]domain.Dependency{},
		drafts:       map[string]domain.BusinessAgentDraft{},
		components:   map[string][]domain.CompositionComponent{},
		seqOf:        map[string]int64{},
		nowFn:        func() time.Time { return time.Now().UTC() },
	}
}

func (m *Memory) Sources() SourceRepo          { return (*memorySources)(m) }
func (m *Memory) Candidates() CandidateRepo    { return (*memoryCandidates)(m) }
func (m *Memory) Capabilities() CapabilityRepo { return (*memoryCapabilities)(m) }
func (m *Memory) Dependencies() DependencyRepo { return (*memoryDependencies)(m) }
func (m *Memory) Drafts() DraftRepo            { return (*memoryDrafts)(m) }
func (m *Memory) Ping(context.Context) error   { return nil }
func (m *Memory) Close()                       { m.mu.Lock(); m.closed = true; m.mu.Unlock() }

// order assigns (or reuses) the monotonic sort key for a record id.
func (m *Memory) order(id string) int64 {
	if existing, ok := m.seqOf[id]; ok {
		return existing
	}
	m.seq++
	m.seqOf[id] = m.seq
	return m.seq
}

type memorySources Memory

func (s *memorySources) base() *Memory { return (*Memory)(s) }

func (s *memorySources) Create(_ context.Context, src domain.ExternalSource) (domain.ExternalSource, error) {
	m := s.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	if src.ID == "" {
		src.ID = domain.NewID(src.Name)
	}
	if _, exists := m.sources[src.ID]; exists {
		return domain.ExternalSource{}, fmt.Errorf("%w: source %s already exists", ErrConflict, src.ID)
	}
	now := m.nowFn()
	src.CreatedAt, src.UpdatedAt = now, now
	if src.Status == "" {
		src.Status = "active"
	}
	m.order(src.ID)
	m.sources[src.ID] = src
	return src, nil
}

func (s *memorySources) Get(_ context.Context, id string) (domain.ExternalSource, error) {
	m := s.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	src, ok := m.sources[id]
	if !ok {
		return domain.ExternalSource{}, fmt.Errorf("%w: source %s", ErrNotFound, id)
	}
	return src, nil
}

func (s *memorySources) List(_ context.Context) ([]domain.ExternalSource, error) {
	m := s.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.ExternalSource, 0, len(m.sources))
	for _, src := range m.sources {
		out = append(out, src)
	}
	sort.Slice(out, func(i, j int) bool { return m.seqOf[out[i].ID] < m.seqOf[out[j].ID] })
	return out, nil
}

func (s *memorySources) EnsureByName(_ context.Context, src domain.ExternalSource) (domain.ExternalSource, error) {
	m := s.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.sources {
		if existing.Name == src.Name && existing.Type == src.Type {
			return existing, nil
		}
	}
	if src.ID == "" {
		src.ID = domain.NewID(src.Name)
	}
	now := m.nowFn()
	src.CreatedAt, src.UpdatedAt = now, now
	if src.Status == "" {
		src.Status = "active"
	}
	m.order(src.ID)
	m.sources[src.ID] = src
	return src, nil
}

type memoryCandidates Memory

func (c *memoryCandidates) base() *Memory { return (*Memory)(c) }

func (c *memoryCandidates) Upsert(_ context.Context, cand domain.CapabilityCandidate) (domain.CapabilityCandidate, error) {
	m := c.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.nowFn()
	// (source_id, external_id) is the natural key: a re-import refreshes in place.
	for id, existing := range m.candidates {
		if existing.SourceID == cand.SourceID && existing.ExternalID == cand.ExternalID {
			if !existing.Status.Pending() {
				// Terminal candidates are never silently rewritten by a re-import.
				return existing, nil
			}
			cand.ID = id
			cand.CreatedAt = existing.CreatedAt
			// A reviewer's edits survive re-extraction.
			cand.Review = existing.Review
			cand.UpdatedAt = now
			m.candidates[id] = cand
			return cand, nil
		}
	}
	if cand.ID == "" {
		cand.ID = domain.NewID(cand.Name)
	}
	cand.CreatedAt, cand.UpdatedAt = now, now
	m.order(cand.ID)
	m.candidates[cand.ID] = cand
	return cand, nil
}

func (c *memoryCandidates) Get(_ context.Context, id string) (domain.CapabilityCandidate, error) {
	m := c.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	cand, ok := m.candidates[id]
	if !ok {
		return domain.CapabilityCandidate{}, fmt.Errorf("%w: candidate %s", ErrNotFound, id)
	}
	return cand, nil
}

func (c *memoryCandidates) List(_ context.Context, f CandidateFilter) ([]domain.CapabilityCandidate, Page, error) {
	m := c.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []domain.CapabilityCandidate
	for _, cand := range m.candidates {
		if f.Status != "" && string(cand.Status) != f.Status {
			continue
		}
		if f.Type != "" && string(cand.CandidateType) != f.Type {
			continue
		}
		if f.SourceID != "" && cand.SourceID != f.SourceID {
			continue
		}
		if f.Query != "" && !matchesText(f.Query, cand.Name, cand.Description, cand.ExternalID) {
			continue
		}
		all = append(all, cand)
	}
	sort.Slice(all, func(i, j int) bool { return m.seqOf[all[i].ID] < m.seqOf[all[j].ID] })
	page := Page{Total: len(all), Limit: f.Limit, Offset: f.Offset}
	page.Normalize()
	return sliceWindow(all, page), page, nil
}

func (c *memoryCandidates) Update(_ context.Context, cand domain.CapabilityCandidate) (domain.CapabilityCandidate, error) {
	m := c.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.candidates[cand.ID]
	if !ok {
		return domain.CapabilityCandidate{}, fmt.Errorf("%w: candidate %s", ErrNotFound, cand.ID)
	}
	cand.CreatedAt = existing.CreatedAt
	cand.UpdatedAt = m.nowFn()
	m.candidates[cand.ID] = cand
	return cand, nil
}

type memoryCapabilities Memory

func (c *memoryCapabilities) base() *Memory { return (*Memory)(c) }

func (c *memoryCapabilities) Create(_ context.Context, capability domain.Capability) (domain.Capability, error) {
	m := c.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.capabilities {
		if existing.Type == capability.Type && existing.Slug == capability.Slug {
			return domain.Capability{}, fmt.Errorf("%w: capability %s/%s already registered", ErrConflict, capability.Type, capability.Slug)
		}
	}
	if capability.ID == "" {
		capability.ID = domain.NewID(capability.Slug)
	}
	now := m.nowFn()
	capability.CreatedAt, capability.UpdatedAt = now, now
	m.order(capability.ID)
	m.capabilities[capability.ID] = capability
	return capability, nil
}

func (c *memoryCapabilities) Get(_ context.Context, id string) (domain.Capability, error) {
	m := c.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	capability, ok := m.capabilities[id]
	if !ok {
		return domain.Capability{}, fmt.Errorf("%w: capability %s", ErrNotFound, id)
	}
	return capability, nil
}

func (c *memoryCapabilities) GetBySlug(_ context.Context, capType domain.CapabilityType, slug string) (domain.Capability, error) {
	m := c.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, capability := range m.capabilities {
		if capability.Type == capType && capability.Slug == slug {
			return capability, nil
		}
	}
	return domain.Capability{}, fmt.Errorf("%w: capability %s/%s", ErrNotFound, capType, slug)
}

func (c *memoryCapabilities) FindByExternalID(_ context.Context, sourceSystem, externalID string) (domain.Capability, error) {
	m := c.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	var fallback *domain.Capability
	for id, capability := range m.capabilities {
		if capability.ExternalID != externalID {
			continue
		}
		if capability.SourceSystem == sourceSystem {
			return m.capabilities[id], nil
		}
		copyCap := capability
		fallback = &copyCap
	}
	if fallback != nil {
		// The same asset imported from a different source still satisfies the reference.
		return *fallback, nil
	}
	return domain.Capability{}, fmt.Errorf("%w: capability external_id=%s", ErrNotFound, externalID)
}

func (c *memoryCapabilities) List(_ context.Context, f CapabilityFilter) ([]domain.Capability, Page, error) {
	m := c.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []domain.Capability
	for _, capability := range m.capabilities {
		if f.Type != "" && string(capability.Type) != f.Type {
			continue
		}
		if f.Status != "" && string(capability.Status) != f.Status {
			continue
		}
		if f.Domain != "" && capability.BusinessDomain != f.Domain {
			continue
		}
		if f.Query != "" && !matchesText(f.Query, append([]string{capability.Name, capability.Description, capability.Slug}, append(capability.Intents, capability.Tags...)...)...) {
			continue
		}
		all = append(all, capability)
	}
	sort.Slice(all, func(i, j int) bool { return m.seqOf[all[i].ID] < m.seqOf[all[j].ID] })
	page := Page{Total: len(all), Limit: f.Limit, Offset: f.Offset}
	page.Normalize()
	return sliceWindow(all, page), page, nil
}

func (c *memoryCapabilities) Update(_ context.Context, capability domain.Capability) (domain.Capability, error) {
	m := c.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.capabilities[capability.ID]
	if !ok {
		return domain.Capability{}, fmt.Errorf("%w: capability %s", ErrNotFound, capability.ID)
	}
	for id, other := range m.capabilities {
		if id != capability.ID && other.Type == capability.Type && other.Slug == capability.Slug {
			return domain.Capability{}, fmt.Errorf("%w: capability %s/%s already registered", ErrConflict, capability.Type, capability.Slug)
		}
	}
	capability.CreatedAt = existing.CreatedAt
	capability.UpdatedAt = m.nowFn()
	m.capabilities[capability.ID] = capability
	return capability, nil
}

type memoryDependencies Memory

func (d *memoryDependencies) base() *Memory { return (*Memory)(d) }

func (d *memoryDependencies) Replace(_ context.Context, capabilityID string, deps []domain.Dependency) error {
	m := d.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.nowFn()
	stored := make([]domain.Dependency, 0, len(deps))
	for _, dep := range deps {
		if dep.ID == "" {
			dep.ID = domain.NewID("dep")
		}
		dep.CapabilityID = capabilityID
		dep.CreatedAt = now
		stored = append(stored, dep)
	}
	if len(stored) == 0 {
		delete(m.dependencies, capabilityID)
		return nil
	}
	m.dependencies[capabilityID] = stored
	return nil
}

func (d *memoryDependencies) ListFor(_ context.Context, capabilityID string) ([]domain.Dependency, error) {
	m := d.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	deps := m.dependencies[capabilityID]
	out := make([]domain.Dependency, 0, len(deps))
	for _, dep := range deps {
		if target, ok := m.capabilities[dep.DependsOnCapabilityID]; ok {
			dep.DependsOnSlug = target.Slug
			dep.DependsOnType = target.Type
			dep.DependsOnName = target.Name
		}
		out = append(out, dep)
	}
	return out, nil
}

type memoryDrafts Memory

func (d *memoryDrafts) base() *Memory { return (*Memory)(d) }

func (d *memoryDrafts) Create(_ context.Context, draft domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error) {
	m := d.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	if draft.ID == "" {
		draft.ID = domain.NewID(draft.Slug)
	}
	now := m.nowFn()
	draft.CreatedAt, draft.UpdatedAt = now, now
	m.order(draft.ID)
	m.drafts[draft.ID] = draft
	m.components[draft.ID] = stampComponents(draft.ID, components, now)
	return draft, nil
}

func (d *memoryDrafts) Get(_ context.Context, id string) (domain.BusinessAgentDraft, error) {
	m := d.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	draft, ok := m.drafts[id]
	if !ok {
		return domain.BusinessAgentDraft{}, fmt.Errorf("%w: draft %s", ErrNotFound, id)
	}
	draft.Components = m.hydrateComponents(m.components[id])
	return draft, nil
}

func (d *memoryDrafts) List(_ context.Context, f DraftFilter) ([]domain.BusinessAgentDraft, Page, error) {
	m := d.base()
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []domain.BusinessAgentDraft
	for _, draft := range m.drafts {
		if f.Status != "" && string(draft.Status) != f.Status {
			continue
		}
		if f.Query != "" && !matchesText(f.Query, draft.Name, draft.Goal, draft.Slug) {
			continue
		}
		all = append(all, draft)
	}
	sort.Slice(all, func(i, j int) bool { return m.seqOf[all[i].ID] < m.seqOf[all[j].ID] })
	page := Page{Total: len(all), Limit: f.Limit, Offset: f.Offset}
	page.Normalize()
	return sliceWindow(all, page), page, nil
}

func (d *memoryDrafts) Update(_ context.Context, draft domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error) {
	m := d.base()
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.drafts[draft.ID]
	if !ok {
		return domain.BusinessAgentDraft{}, fmt.Errorf("%w: draft %s", ErrNotFound, draft.ID)
	}
	now := m.nowFn()
	draft.CreatedAt = existing.CreatedAt
	draft.UpdatedAt = now
	m.drafts[draft.ID] = draft
	if components != nil {
		m.components[draft.ID] = stampComponents(draft.ID, components, now)
	}
	draft.Components = m.hydrateComponents(m.components[draft.ID])
	return draft, nil
}

func (m *Memory) hydrateComponents(components []domain.CompositionComponent) []domain.CompositionComponent {
	out := make([]domain.CompositionComponent, 0, len(components))
	for _, comp := range components {
		if capability, ok := m.capabilities[comp.CapabilityID]; ok {
			comp.CapabilitySlug = capability.Slug
			comp.CapabilityName = capability.Name
			comp.CapabilityType = capability.Type
		}
		out = append(out, comp)
	}
	return out
}

func stampComponents(draftID string, components []domain.CompositionComponent, now time.Time) []domain.CompositionComponent {
	out := make([]domain.CompositionComponent, 0, len(components))
	for i, comp := range components {
		if comp.ID == "" {
			comp.ID = domain.NewID("component")
		}
		comp.DraftID = draftID
		comp.OrderIndex = i
		comp.CreatedAt = now
		out = append(out, comp)
	}
	return out
}

func matchesText(query string, fields ...string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func sliceWindow[T any](items []T, page Page) []T {
	if page.Offset >= len(items) {
		return []T{}
	}
	end := page.Offset + page.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[page.Offset:end]
}
