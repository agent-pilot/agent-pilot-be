package agentcontext

import (
	"agent-pilot-be/internal/agent/selector"
	"agent-pilot-be/internal/llm"
	"agent-pilot-be/internal/runtime"
	"context"
	"strings"
)

type RuntimeRef struct {
	Skill string
	Path  string
}

type Builder struct {
	rt          *runtime.Runtime
	refSelector *selector.ReferenceSelector
}

func NewBuilder(rt *runtime.Runtime, refSelector *selector.ReferenceSelector) *Builder {
	return &Builder{
		rt:          rt,
		refSelector: refSelector,
	}
}

// SkillContext ensures SKILL.md is loaded for chosen skills and builds the skill context string.
func (b *Builder) SkillContext(chosen []string) (string, error) {
	if b == nil || b.rt == nil || b.rt.Skills == nil {
		return "", nil
	}
	if err := b.rt.Skills.EnsureFullContent(chosen); err != nil {
		return "", err
	}
	var out strings.Builder
	for i := range b.rt.Skills.Skills {
		s := &b.rt.Skills.Skills[i]
		name := strings.TrimSpace(s.Meta.Name)
		if name == "" {
			name = strings.TrimSpace(s.Name)
		}
		if !contains(chosen, name) {
			continue
		}
		_ = s.LoadFull()
		out.WriteString("\n==== " + name + " ====\n")
		out.WriteString(s.Content)
		out.WriteString("\n")
	}
	return out.String(), nil
}

// ReferenceCatalog lists available reference paths for chosen skills.
func (b *Builder) ReferenceCatalog(chosenSkills []string) (string, error) {
	if b == nil || b.rt == nil || b.rt.Skills == nil {
		return "", nil
	}
	if err := b.rt.Skills.EnsureReferenceIndex(chosenSkills); err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("Available reference files (paths are relative to each skill directory):\n")
	for i := range b.rt.Skills.Skills {
		s := &b.rt.Skills.Skills[i]
		name := strings.TrimSpace(s.Meta.Name)
		if name == "" {
			name = strings.TrimSpace(s.Name)
		}
		if !contains(chosenSkills, name) {
			continue
		}
		if len(s.References) == 0 {
			continue
		}
		out.WriteString("- " + name + ":\n")
		for _, r := range s.References {
			out.WriteString("  - " + r.Path + "\n")
		}
	}
	return out.String(), nil
}

func (b *Builder) References(
	ctx context.Context,
	chosenSkills []string,
	skillCtx string,
	messages []llm.Message,
) (refCtx string, picked []RuntimeRef, raw string, err error) {
	if b == nil || b.rt == nil || b.rt.Skills == nil {
		return "", nil, "", nil
	}
	if b.refSelector == nil {
		return "", nil, "", nil
	}

	refCatalog, err := b.ReferenceCatalog(chosenSkills)
	if err != nil {
		return "", nil, "", err
	}
	refs, raw, err := b.refSelector.Select(ctx, skillCtx, refCatalog, messages)
	if err != nil {
		return "", nil, raw, err
	}
	picked = make([]RuntimeRef, 0, len(refs))
	for _, r := range refs {
		picked = append(picked, RuntimeRef{Skill: r.Skill, Path: r.Path})
	}
	refCtx, err = b.loadReferenceContext(chosenSkills, picked)
	if err != nil {
		return "", picked, raw, err
	}
	return refCtx, picked, raw, nil
}

func (b *Builder) loadReferenceContext(chosenSkills []string, picked []RuntimeRef) (string, error) {
	if b == nil || b.rt == nil || b.rt.Skills == nil {
		return "", nil
	}
	if err := b.rt.Skills.EnsureReferenceIndex(chosenSkills); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, rr := range picked {
		skillName := strings.TrimSpace(rr.Skill)
		refPath := strings.TrimSpace(rr.Path)
		if skillName == "" || refPath == "" {
			continue
		}
		for i := range b.rt.Skills.Skills {
			s := &b.rt.Skills.Skills[i]
			name := strings.TrimSpace(s.Meta.Name)
			if name == "" {
				name = strings.TrimSpace(s.Name)
			}
			if name != skillName {
				continue
			}
			content, err := s.LoadReference(refPath)
			if err != nil {
				continue
			}
			out.WriteString("\n==== " + skillName + " / " + refPath + " ====\n")
			out.WriteString(content)
			out.WriteString("\n")
		}
	}
	return out.String(), nil
}

func contains(list []string, s string) bool {
	s = strings.TrimSpace(s)
	for _, x := range list {
		if strings.TrimSpace(x) == s {
			return true
		}
	}
	return false
}
