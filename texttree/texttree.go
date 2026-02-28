package texttree

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func ParseFile(path string) (map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	return ParseReader(f)
}

func ParseReader(r io.Reader) (map[string][]string, error) {
	children := map[string][]string{} // parent name to its direct children
	seen := map[string]struct{}{}
	parents := map[string]string{} // tracks which parent a child belongs to (for duplicate detection)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parentName, childName, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}

		parentName = strings.TrimSpace(parentName)
		childName = strings.TrimSpace(childName)
		if parentName == "" || childName == "" {
			continue
		}

		if _, ok := parents[childName]; ok {
			return nil, fmt.Errorf("duplicate child in tree: %q", childName)
		}

		parents[childName] = parentName
		children[parentName] = append(children[parentName], childName)
		seen[parentName] = struct{}{}
		seen[childName] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading tree: %w", err)
	}

	for name := range seen {
		if err := checkCycle(name, parents); err != nil {
			return nil, err
		}
	}

	tree := map[string][]string{}
	for name := range seen {
		tree[name] = collectDescendants(name, children)
	}

	return tree, nil
}

func checkCycle(name string, parents map[string]string) error {
	var visited = map[string]bool{}
	cur := name
	for {
		if visited[cur] {
			return fmt.Errorf("cycle detected in tree at %q", cur)
		}
		visited[cur] = true
		p, ok := parents[cur]
		if !ok {
			break
		}
		cur = p
	}
	return nil
}

func collectDescendants(name string, children map[string][]string) []string {
	var result []string
	var stack []string
	stack = append(stack, name)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result = append(result, cur)
		stack = append(stack, children[cur]...)
	}
	return result
}
