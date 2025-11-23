package api

import (
	"fmt"
	"slices"

	"golang.org/x/net/html"
)

func findOneNode(root *html.Node, query func(*html.Node) bool) (*html.Node, error) {
	queue := []*html.Node{root}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if query(node) {
			return node, nil
		}

		queue = slices.AppendSeq(queue, node.ChildNodes())
	}
	return nil, fmt.Errorf("node not found")
}

// NOTE: walkdir みたいにこのノード配下は skip などできると良いかも。
func findAllNodes(root *html.Node, query func(*html.Node) bool) []*html.Node {
	queue := []*html.Node{root}
	nodes := []*html.Node{}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if query(node) {
			nodes = append(nodes, node)
		}

		queue = slices.AppendSeq(queue, node.ChildNodes())
	}
	return nodes
}

func attrIs(n *html.Node, k, v string) bool {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val == v
		}
	}
	return false
}

func getAttr(n *html.Node, k string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val, true
		}
	}
	return "", false
}
