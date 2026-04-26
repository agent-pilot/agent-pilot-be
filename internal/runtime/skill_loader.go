package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Skill 下载skills的方式：
// npx skills add larksuite/cli -g -y
type Skill struct {
	Name        string
	Dir         string
	SkillMDPath string
	Meta        Meta
	// Content is the full SKILL.md content. It is loaded lazily for selected skills.
	Content    string
	References []Reference

	referencesIndexed bool
}

type Meta struct {
	Name        string
	Version     string
	Description string
	Metadata    MetaMetadata
}

type MetaMetadata struct {
	Requires MetaRequires
	CLIHelp  string
}

type MetaRequires struct {
	Bins []string
}

type listItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// 从文件系统中加载出可用的skills（只记录元数据）
func loadViaNpx(ctx context.Context) ([]Skill, error) {
	cctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	result := RunCLI(cctx, "npx", []string{"skills", "list", "-g", "--json"})
	if result.Err != nil {
		return nil, fmt.Errorf("npx skills list failed: %v; stderr=%s", result.Err, strings.TrimSpace(string(result.Stderr)))
	}

	var items []listItem
	if err := json.Unmarshal(result.Stdout, &items); err != nil {
		return nil, fmt.Errorf("parse skills list json: %w", err)
	}

	var out []Skill
	for _, it := range items {
		if !strings.HasPrefix(it.Name, "lark-") {
			continue
		}
		md := filepath.Join(it.Path, "SKILL.md")
		s, err := readSkillPreview(it.Name, it.Path, md)
		if err != nil {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func readSkillPreview(name, dir, mdPath string) (Skill, error) {
	b, err := os.ReadFile(mdPath)
	if err != nil {
		return Skill{}, err
	}
	full := string(b)
	meta, _, _ := parseSkillMD(full)
	return Skill{
		Name:        name,
		Dir:         dir,
		SkillMDPath: mdPath,
		Meta:        meta,
	}, nil
}

// LoadFull 获取整个SKILL.md的内容
func (s *Skill) LoadFull() error {
	if s == nil {
		return errors.New("nil skill")
	}
	if s.Content != "" {
		return nil
	}
	b, err := os.ReadFile(s.SkillMDPath)
	if err != nil {
		return err
	}
	s.Content = string(b)
	// refresh meta from full content in case preview truncation cut it (shouldn't, but safe)
	meta, _, _ := parseSkillMD(s.Content)
	if meta.Name != "" || meta.Description != "" || meta.Version != "" {
		s.Meta = meta
	}
	return nil
}

// parseSkillMD parses SKILL.md frontmatter (--- ... ---) into Meta and returns (meta, body, ok).
func parseSkillMD(full string) (Meta, string, bool) {
	lines := strings.Split(full, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return Meta{}, full, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return Meta{}, full, false
	}
	fm := lines[1:end]
	body := strings.Join(lines[end+1:], "\n")

	var meta Meta
	var section string
	var subSection string

	for _, ln := range fm {
		raw := ln
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))

		if indent == 0 {
			section = ""
			subSection = ""
			k, v, ok := splitKV(trim)
			if !ok {
				continue
			}
			switch k {
			case "name":
				meta.Name = unquote(v)
			case "version":
				meta.Version = unquote(v)
			case "description":
				meta.Description = unquote(v)
			case "metadata":
				section = "metadata"
			}
			continue
		}

		// metadata children (indent 2)
		if indent == 2 && section == "metadata" {
			k, v, ok := splitKV(trim)
			if !ok {
				// allow "requires:" with empty v
				if strings.HasSuffix(trim, ":") {
					k = strings.TrimSuffix(trim, ":")
					v = ""
					ok = true
				}
			}
			if !ok {
				continue
			}
			switch k {
			case "cliHelp":
				meta.Metadata.CLIHelp = unquote(v)
			case "requires":
				subSection = "requires"
			}
			continue
		}

		// metadata.requires children (indent 4)
		if indent == 4 && section == "metadata" && subSection == "requires" {
			k, v, ok := splitKV(trim)
			if !ok {
				continue
			}
			switch k {
			case "bins":
				meta.Metadata.Requires.Bins = parseInlineStringList(v)
			}
			continue
		}
	}

	return meta, body, true
}

func splitKV(s string) (k, v string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", "", false
	}
	k = strings.TrimSpace(s[:i])
	v = strings.TrimSpace(s[i+1:])
	return k, v, k != ""
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseInlineStringList(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	// Expect: ["a","b"] or ["a", "b"]
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		inner := strings.TrimSpace(v[1 : len(v)-1])
		if inner == "" {
			return nil
		}
		parts := strings.Split(inner, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			p = strings.Trim(p, `"'`)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return []string{strings.Trim(v, `"'`)}
}
