package ginai

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Registry stores explicitly registered tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

func (r *Registry) Register(tool *Tool) error {
	return r.register(tool, false)
}

func (r *Registry) Upsert(tool *Tool) error {
	return r.register(tool, true)
}

func (r *Registry) register(tool *Tool, allowUpdate bool) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}
	if strings.TrimSpace(tool.Name) == "" {
		return fmt.Errorf("tool name is required")
	}
	if strings.EqualFold(tool.Method, "DELETE") {
		return fmt.Errorf("DELETE tools are disabled in v0.1")
	}
	if !tool.ReadOnly && !tool.NeedConfirm {
		tool.NeedConfirm = true
	}
	if tool.Schema == nil {
		schema, err := SchemaFromStruct(tool.Params)
		if err != nil {
			return err
		}
		tool.Schema = schema
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[tool.Name]; ok && !allowUpdate {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}

	cp := *tool
	cp.Roles = slices.Clone(tool.Roles)
	cp.AllowFields = slices.Clone(tool.AllowFields)
	r.tools[cp.Name] = &cp
	return nil
}

func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok {
		return nil, false
	}
	cp := *tool
	cp.Roles = slices.Clone(tool.Roles)
	cp.AllowFields = slices.Clone(tool.AllowFields)
	return &cp, true
}

func (r *Registry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		cp := *tool
		cp.Roles = slices.Clone(tool.Roles)
		cp.AllowFields = slices.Clone(tool.AllowFields)
		out = append(out, &cp)
	}
	slices.SortFunc(out, func(a, b *Tool) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func (r *Registry) ExportSchemas() []LLMToolSchema {
	tools := r.List()
	out := make([]LLMToolSchema, 0, len(tools))
	for _, tool := range tools {
		out = append(out, LLMToolSchema{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Schema,
		})
	}
	return out
}
