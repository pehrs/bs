package main

import "strings"

// ── List item ─────────────────────────────────────────────────────────────────

type entityItem struct{ entity Entity }

func (i entityItem) Title() string {
	if i.entity.Metadata.Title != "" {
		return i.entity.Metadata.Title
	}
	return i.entity.Metadata.Name
}

func (i entityItem) Description() string {
	parts := []string{i.entity.Kind}
	if ns := i.entity.Metadata.Namespace; ns != "" && ns != "default" {
		parts = append(parts, ns)
	}
	if d := i.entity.Metadata.Description; d != "" {
		if len(d) > 72 {
			d = d[:69] + "..."
		}
		parts = append(parts, d)
	}
	return strings.Join(parts, " · ")
}

func (i entityItem) FilterValue() string {
	return i.entity.Kind + "/" + i.entity.Metadata.Namespace + "/" + i.entity.Metadata.Name
}
