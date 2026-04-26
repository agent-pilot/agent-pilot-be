package runtime

import (
	"context"
	"fmt"
	"strings"
)

type Registry struct {
	Skills []Skill
}

// NewRegistry 通过npx skills list -g --json加载可用所有的skills
func NewRegistry(ctx context.Context) *Registry {
	skills, err := loadViaNpx(ctx)
	if err == nil && len(skills) > 0 {
		return &Registry{
			Skills: skills,
		}
	}
	panic(fmt.Errorf("注册skills失败：%v", err))
}

// EnsureFullContent 根据skillName获取整个skill文档
func (r *Registry) EnsureFullContent(skillNames []string) error {
	if r == nil {
		return fmt.Errorf("nil registry")
	}
	if len(skillNames) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, n := range skillNames {
		want[strings.TrimSpace(n)] = true
	}

	for i := range r.Skills {
		s := &r.Skills[i]
		name := s.Meta.Name
		if strings.TrimSpace(name) == "" {
			name = s.Name
		}
		if !want[name] {
			continue
		}
		if err := s.LoadFull(); err != nil {
			return err
		}
	}
	return nil
}

// EnsureReferenceIndex scans references/ directory for selected skills.
// It only builds an index of reference file paths (no content is loaded).
func (r *Registry) EnsureReferenceIndex(skillNames []string) error {
	if r == nil {
		return fmt.Errorf("nil registry")
	}
	if len(skillNames) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, n := range skillNames {
		want[strings.TrimSpace(n)] = true
	}

	for i := range r.Skills {
		s := &r.Skills[i]
		name := s.Meta.Name
		if strings.TrimSpace(name) == "" {
			name = s.Name
		}
		if !want[name] {
			continue
		}
		if err := s.EnsureReferenceIndex(); err != nil {
			return err
		}
	}
	return nil
}
