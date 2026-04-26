package runtime

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Reference struct {
	Path    string
	Content string
	Type    string
}

func buildReferenceIndex(skillDir string) ([]Reference, error) {
	base := filepath.Join(skillDir, "references")
	st, err := os.Stat(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}

	var out []Reference
	err = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden dirs.
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		switch ext {
		case ".md", ".markdown":
			// ok
		default:
			return nil
		}
		rel, err := filepath.Rel(skillDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		out = append(out, Reference{
			Path: rel,
			Type: strings.TrimPrefix(ext, "."),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (s *Skill) EnsureReferenceIndex() error {
	if s == nil {
		return fmt.Errorf("nil skill")
	}
	if s.referencesIndexed {
		return nil
	}
	refs, err := buildReferenceIndex(s.Dir)
	if err != nil {
		return err
	}
	s.References = refs
	s.referencesIndexed = true
	return nil
}

func (s *Skill) LoadReference(relPath string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("nil skill")
	}
	if err := s.EnsureReferenceIndex(); err != nil {
		return "", err
	}
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	if relPath == "" {
		return "", fmt.Errorf("empty reference path")
	}
	if !strings.HasPrefix(relPath, "references/") {
		return "", fmt.Errorf("reference path must start with references/: %s", relPath)
	}

	full := filepath.Join(s.Dir, filepath.FromSlash(relPath))
	full = filepath.Clean(full)
	root := filepath.Clean(s.Dir) + string(os.PathSeparator)
	if !strings.HasPrefix(full+string(os.PathSeparator), root) && full != filepath.Clean(s.Dir) {
		return "", fmt.Errorf("reference path escapes skill dir: %s", relPath)
	}

	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
