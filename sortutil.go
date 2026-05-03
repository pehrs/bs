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
