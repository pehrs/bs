package main

import (
	"sort"

	"github.com/charmbracelet/bubbles/list"
)

type sortOrder int

const (
	sortByName      sortOrder = iota
	sortByKind
	sortByNamespace
	numSortOrders
)

var sortOrderLabels = [numSortOrders]string{"name", "kind", "namespace"}

// sortItems returns a sorted copy of items; original slice is not modified.
func sortItems(items []list.Item, order sortOrder) []list.Item {
	out := make([]list.Item, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].(entityItem).entity
		b := out[j].(entityItem).entity
		switch order {
		case sortByKind:
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.Metadata.Name < b.Metadata.Name
		case sortByNamespace:
			if a.Metadata.Namespace != b.Metadata.Namespace {
				return a.Metadata.Namespace < b.Metadata.Namespace
			}
			return a.Metadata.Name < b.Metadata.Name
		default: // sortByName
			return a.Metadata.Name < b.Metadata.Name
		}
	})
	return out
}
