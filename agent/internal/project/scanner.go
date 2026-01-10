package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ProjectType string

const (
	TypeGit     ProjectType = "git"
	TypeGo      ProjectType = "go"
	TypeNode    ProjectType = "node"
	TypePython  ProjectType = "python"
	TypeRust    ProjectType = "rust"
	TypeUnknown ProjectType = "unknown"
)

type Project struct {
	Path string      `json:"path"`
	Name string      `json:"name"`
	Type ProjectType `json:"type"`
}

type Scanner struct {
	workspaces []string
	maxDepth   int
}

func NewScanner(workspaces []string) *Scanner {
	return &Scanner{
		workspaces: workspaces,
		maxDepth:   2,
	}
}

func (s *Scanner) Scan() ([]Project, error) {
	var projects []Project
	seen := make(map[string]bool)

	for _, ws := range s.workspaces {
		absWs, err := filepath.Abs(ws)
		if err != nil {
			continue
		}

		err = s.scanDir(absWs, 0, &projects, seen)
		if err != nil {
			continue
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})

	return projects, nil
}

func (s *Scanner) scanDir(dir string, depth int, projects *[]Project, seen map[string]bool) error {
	if depth > s.maxDepth {
		return nil
	}

	if seen[dir] {
		return nil
	}

	if s.IsProject(dir) {
		seen[dir] = true
		*projects = append(*projects, Project{
			Path: dir,
			Name: filepath.Base(dir),
			Type: s.DetectType(dir),
		})
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
			continue
		}

		subDir := filepath.Join(dir, name)
		s.scanDir(subDir, depth+1, projects, seen)
	}

	return nil
}

func (s *Scanner) IsProject(path string) bool {
	indicators := []string{
		".git",
		"go.mod",
		"package.json",
		"Cargo.toml",
		"pyproject.toml",
		"setup.py",
		"requirements.txt",
	}

	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			return true
		}
	}

	return false
}

func (s *Scanner) DetectType(path string) ProjectType {
	checks := []struct {
		file     string
		projType ProjectType
	}{
		{"go.mod", TypeGo},
		{"Cargo.toml", TypeRust},
		{"package.json", TypeNode},
		{"pyproject.toml", TypePython},
		{"setup.py", TypePython},
		{"requirements.txt", TypePython},
		{".git", TypeGit},
	}

	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(path, check.file)); err == nil {
			return check.projType
		}
	}

	return TypeUnknown
}

func (s *Scanner) ValidatePath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return os.ErrNotExist
	}

	return nil
}
