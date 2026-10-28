package menu

import "sort"

// BuildTree arranges flat items into a hierarchy. Siblings are sorted by
// position, then id. When onlyEnabled is true, disabled items are dropped
// together with all their descendants.
func BuildTree(items []Item, onlyEnabled bool) []*Node {
	nodes := make(map[int64]*Node, len(items))
	for _, item := range items {
		nodes[item.ID] = &Node{Item: item, Items: []*Node{}}
	}

	roots := []*Node{}
	for _, item := range items {
		node := nodes[item.ID]
		if item.ParentID == nil {
			roots = append(roots, node)
			continue
		}
		if parent, ok := nodes[*item.ParentID]; ok {
			parent.Items = append(parent.Items, node)
		}
	}

	if onlyEnabled {
		roots = pruneDisabled(roots)
	}
	sortNodes(roots)
	return roots
}

func pruneDisabled(nodes []*Node) []*Node {
	kept := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		n.Items = pruneDisabled(n.Items)
		kept = append(kept, n)
	}
	return kept
}

func sortNodes(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Position != nodes[j].Position {
			return nodes[i].Position < nodes[j].Position
		}
		return nodes[i].ID < nodes[j].ID
	})
	for _, n := range nodes {
		sortNodes(n.Items)
	}
}

// checkNoCycles applies moves to the current parent map and verifies that the
// final hierarchy is acyclic and that every referenced item exists.
func checkNoCycles(items []Item, moves []Move) error {
	parents := make(map[int64]*int64, len(items))
	for _, item := range items {
		parents[item.ID] = item.ParentID
	}

	for _, m := range moves {
		if _, ok := parents[m.ID]; !ok {
			return invalid("id", "item %d does not exist", m.ID)
		}
		if m.ParentID != nil {
			if _, ok := parents[*m.ParentID]; !ok {
				return invalid("parent_id", "parent %d does not exist", *m.ParentID)
			}
		}
		parents[m.ID] = m.ParentID
	}

	for id := range parents {
		seen := map[int64]bool{id: true}
		for p := parents[id]; p != nil; p = parents[*p] {
			if seen[*p] {
				return invalid("parent_id", "moving item %d would create a cycle", id)
			}
			seen[*p] = true
		}
	}
	return nil
}
