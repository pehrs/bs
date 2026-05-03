package main

import (
	"fmt"
	"sort"
	"strings"
)

// ── Entity types ──────────────────────────────────────────────────────────────

type Entity struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   EntityMetadata         `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
	Relations  []Relation             `json:"relations"`
}

type EntityMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	UID         string            `json:"uid"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Tags        []string          `json:"tags"`
}

type Relation struct {
	TargetRef string `json:"targetRef"`
	Type      string `json:"type"`
}

type pageInfo struct {
	NextCursor string `json:"nextCursor"`
}

type queryResponse struct {
	Items      []Entity `json:"items"`
	TotalItems int      `json:"totalItems"`
	PageInfo   pageInfo `json:"pageInfo"`
}

// ── Detail renderer ───────────────────────────────────────────────────────────

func renderEntityDetail(e Entity) string {
	var sb strings.Builder

	section := func(name string) {
		sb.WriteString("\n")
		sb.WriteString(sectionHeaderStyle.Render("── " + name))
		sb.WriteString("\n")
	}

	field := func(label, value string) {
		if value == "" {
			return
		}
		sb.WriteString(fieldLabelStyle.Render(label))
		sb.WriteString(fieldValueStyle.Render(value))
		sb.WriteString("\n")
	}

	section("Metadata")
	field("name:         ", e.Metadata.Name)
	field("namespace:    ", e.Metadata.Namespace)
	field("uid:          ", e.Metadata.UID)
	field("apiVersion:   ", e.APIVersion)
	if e.Metadata.Title != "" {
		field("title:        ", e.Metadata.Title)
	}
	if e.Metadata.Description != "" {
		field("description:  ", e.Metadata.Description)
	}
	if len(e.Metadata.Tags) > 0 {
		field("tags:         ", strings.Join(e.Metadata.Tags, ", "))
	}

	if len(e.Metadata.Labels) > 0 {
		section("Labels")
		for _, k := range sortedStringKeys(e.Metadata.Labels) {
			field("  "+k+": ", e.Metadata.Labels[k])
		}
	}

	if len(e.Metadata.Annotations) > 0 {
		section("Annotations")
		for _, k := range sortedStringKeys(e.Metadata.Annotations) {
			field("  "+k+": ", e.Metadata.Annotations[k])
		}
	}

	if len(e.Spec) > 0 {
		section("Spec")
		renderSpecMap(&sb, e.Spec, 1)
	}

	if len(e.Relations) > 0 {
		section("Relations")
		byType := make(map[string][]string)
		for _, r := range e.Relations {
			byType[r.Type] = append(byType[r.Type], r.TargetRef)
		}
		relTypes := make([]string, 0, len(byType))
		for t := range byType {
			relTypes = append(relTypes, t)
		}
		sort.Strings(relTypes)
		for _, t := range relTypes {
			field("  "+t+": ", strings.Join(byType[t], "\n             "))
		}
	}

	return sb.String()
}

func renderSpecMap(sb *strings.Builder, m map[string]interface{}, depth int) {
	indent := strings.Repeat("  ", depth)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		switch val := m[k].(type) {
		case map[string]interface{}:
			sb.WriteString(indent)
			sb.WriteString(fieldLabelStyle.Render(k + ":"))
			sb.WriteString("\n")
			renderSpecMap(sb, val, depth+1)
		case []interface{}:
			strs := make([]string, len(val))
			for i, item := range val {
				strs[i] = fmt.Sprintf("%v", item)
			}
			sb.WriteString(indent)
			sb.WriteString(fieldLabelStyle.Render(k + ": "))
			sb.WriteString(fieldValueStyle.Render(strings.Join(strs, ", ")))
			sb.WriteString("\n")
		default:
			sb.WriteString(indent)
			sb.WriteString(fieldLabelStyle.Render(k + ": "))
			sb.WriteString(fieldValueStyle.Render(fmt.Sprintf("%v", val)))
			sb.WriteString("\n")
		}
	}
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
