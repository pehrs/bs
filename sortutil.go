package main

import (
	"sort"

	"github.com/charmbracelet/bubbles/list"
)

type sortOrder int

const (
	sortByName sortOrder = iota
	sortByDescription
	sortByKind
	sortByNamespace
	numSortOrders
)

var sortOrderLabels = [numSortOrders]string{"name", "description", "kind", "namespace"}

// ── Global search sort ────────────────────────────────────────────────────────

type globalSortOrder int

const (
	globalSortByTitle globalSortOrder = iota // alphabetical by document title
	globalSortByKind                         // by document kind, then title
	globalSortByType                         // by result type (software-catalog, techdocs…), then title
	globalSortByRank                         // by API relevance rank
	numGlobalSortOrders
)

var globalSortOrderLabels = [numGlobalSortOrders]string{"title", "kind", "type", "rank"}

// sortGlobalItems returns a sorted copy of globalSearchItem list items.
func sortGlobalItems(items []list.Item, order globalSortOrder, reverse bool) []list.Item {
	out := make([]list.Item, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].(globalSearchItem).result
		b := out[j].(globalSearchItem).result
		var less bool
		switch order {
		case globalSortByKind:
			if a.Document.Kind != b.Document.Kind {
				less = a.Document.Kind < b.Document.Kind
			} else {
				less = a.Document.Title < b.Document.Title
			}
		case globalSortByType:
			if a.Type != b.Type {
				less = a.Type < b.Type
			} else {
				less = a.Document.Title < b.Document.Title
			}
		case globalSortByRank:
			if a.Rank != b.Rank {
				less = a.Rank < b.Rank
			} else {
				less = a.Document.Title < b.Document.Title
			}
		default: // globalSortByTitle
			less = a.Document.Title < b.Document.Title
		}
		if reverse {
			return !less
		}
		return less
	})
	return out
}

// sortItems returns a sorted copy of items; original slice is not modified.
// When reverse is true the order is descending.
func sortItems(items []list.Item, order sortOrder, reverse bool) []list.Item {
	out := make([]list.Item, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].(entityItem).entity
		b := out[j].(entityItem).entity
		var less bool
		switch order {
		case sortByKind:
			if a.Kind != b.Kind {
				less = a.Kind < b.Kind
			} else {
				less = a.Metadata.Name < b.Metadata.Name
			}
		case sortByDescription:
			if a.Metadata.Description != b.Metadata.Description {
				less = a.Metadata.Description < b.Metadata.Description
			} else {
				less = a.Metadata.Name < b.Metadata.Name
			}
		case sortByNamespace:
			if a.Metadata.Namespace != b.Metadata.Namespace {
				less = a.Metadata.Namespace < b.Metadata.Namespace
			} else {
				less = a.Metadata.Name < b.Metadata.Name
			}
		default: // sortByName
			less = a.Metadata.Name < b.Metadata.Name
		}
		if reverse {
			return !less
		}
		return less
	})
	return out
}
