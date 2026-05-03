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
	kindStr := i.entity.Kind
	if t, ok := i.entity.Spec["type"].(string); ok && t != "" && t != "other" {
		kindStr += "/" + t
	}
	parts := []string{kindStr}
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
