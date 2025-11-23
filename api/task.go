package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type TaskSampleIO struct {
	Input  []string
	Output []string
}

type Task struct {
	URL       *url.URL
	Index     string
	SampleIOs []*TaskSampleIO
}

func (c *Client) FetchTaskList(ctx context.Context) ([]*Task, error) {
	taskListURL := &url.URL{
		Scheme: "https",
		Host:   DOMAIN,
		Path:   path.Join("contests", c.family.ContestName(), "tasks"),
	}

	slog.InfoContext(ctx, "fetching task list", slog.String("url", taskListURL.String()))

	req, err := http.NewRequestWithContext(ctx, "GET", taskListURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	root, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	tbody, err := findOneNode(root, func(n *html.Node) bool {
		p := n.Parent
		if p == nil || p.Type != html.ElementNode || p.Data != "table" {
			return false
		}
		return n.Type == html.ElementNode && n.Data == "tbody"
	})
	if err != nil {
		return nil, err
	}

	var taskList []*Task
	for tr := range tbody.ChildNodes() {
		firstTD, err := findOneNode(tr, func(n *html.Node) bool {
			return n.Parent == tr && n.Type == html.ElementNode && n.Data == "td"
		})
		var href, text string
		_, err = findOneNode(tr, func(n *html.Node) bool {
			if n.Type != html.ElementNode || n.Data != "a" || n.Parent != firstTD {
				return false
			}
			a := n

			var ok bool
			href, ok = getAttr(n, "href")
			if !ok {
				return false
			}

			textNode, err := findOneNode(n, func(n *html.Node) bool {
				return n.Parent == a && n.Type == html.TextNode
			})
			if err != nil {
				return false
			}
			text = textNode.Data

			return true
		})
		if err != nil {
			continue
		}
		taskURL, err := taskListURL.Parse(href)
		if err != nil {
			return nil, err
		}
		taskList = append(taskList, &Task{URL: taskURL, Index: text})
		slog.InfoContext(ctx, "found task", slog.String("data", firstTD.Data))
	}
	return taskList, nil
}

func (c *Client) FetchSampleIOs(ctx context.Context, task *Task) error {
	taskURL := task.URL
	slog.InfoContext(ctx, "fetching sample IOs", slog.String("url", taskURL.String()))

	req, err := http.NewRequestWithContext(ctx, "GET", taskURL.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	root, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}

	_ = findAllNodes(root, func(node *html.Node) bool {
		if node.Type != html.ElementNode && node.Data != "section" {
			return false
		}
		section := node

		// section > h3 > (text node) を取得
		numberNode, err := findOneNode(section, func(node *html.Node) bool {
			if node.Type != html.TextNode {
				return false
			}
			p := node.Parent
			if p == nil || p.Type != html.ElementNode || p.Data != "h3" {
				return false
			}
			if node.Parent.Parent != section {
				return false
			}
			return strings.HasPrefix(node.Data, "入力例") ||
				strings.HasPrefix(node.Data, "出力例")
		})
		if err != nil {
			return false
		}
		slog.InfoContext(ctx, "section has example number", slog.String("data", numberNode.Data))

		rawNumber := strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(
			string([]rune(numberNode.Data)[4:]), "出力例"), "入力例"))
		var number int
		if rawNumber != "" {
			number, err = strconv.Atoi(rawNumber)
			if err != nil {
				return false
			}
		} else {
			number = 1
		}
		if len(task.SampleIOs) < number {
			task.SampleIOs = append(task.SampleIOs, make([]*TaskSampleIO, number-len(task.SampleIOs))...)
		}
		isInput := strings.HasPrefix(numberNode.Data, "入力例")
		if task.SampleIOs[number-1] == nil {
			task.SampleIOs[number-1] = &TaskSampleIO{}
		}

		// section > pre > (text node) を取得
		exampleNode, err := findOneNode(section, func(node *html.Node) bool {
			if node.Type != html.TextNode {
				return false
			}
			p := node.Parent
			if p == nil || p.Type != html.ElementNode || p.Data != "pre" {
				return false
			}
			return p.Parent == section
		})
		if err != nil {
			return false
		}
		if isInput {
			task.SampleIOs[number-1].Input = strings.Split(strings.TrimSpace(exampleNode.Data), "\n")
		} else {
			task.SampleIOs[number-1].Output = strings.Split(strings.TrimSpace(exampleNode.Data), "\n")
		}
		return true
	})

	return nil
}
