package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

const DOMAIN = "atcoder.jp"

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

type TaskSampleIO struct {
	Input  []string
	Output []string
}

type Task struct {
	URL       *url.URL
	Index     string
	SampleIOs []*TaskSampleIO
}

func (io *TaskSampleIO) String() string {
	return fmt.Sprintf("Input: %q // Output: %q", io.Input, io.Output)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	taskListURL := &url.URL{
		Scheme: "https",
		Host:   DOMAIN,
		Path:   path.Join("contests", "adt_hard_20251120_3", "tasks", ""),
	}

	tasks, err := fetchTaskList(ctx, taskListURL)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch task list", slog.String("err", err.Error()))
		return
	}
	for _, task := range tasks {
		err = fetchSampleIOs(ctx, task)
		if err != nil {
			slog.ErrorContext(ctx, "failed to fetch sample IOs", slog.String("err", err.Error()))
			return
		}
	}
}

func fetchTaskList(ctx context.Context, taskListURL *url.URL) ([]*Task, error) {
	slog.InfoContext(ctx, "fetching task list", slog.String("url", taskListURL.String()))

	resp, err := http.Get(taskListURL.String())
	if err != nil {
		slog.ErrorContext(ctx, "failed to send GET request", slog.String("url", taskListURL.String()), slog.String("err", err.Error()))
		return nil, err
	}
	defer resp.Body.Close()

	root, err := html.Parse(resp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse HTML", slog.String("url", taskListURL.String()), slog.String("err", err.Error()))
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
		slog.ErrorContext(ctx, "failed to find tbody", slog.String("url", taskListURL.String()), slog.String("err", err.Error()))
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
			slog.ErrorContext(ctx, "failed to parse task URL", slog.String("base_url", taskListURL.String()), slog.String("href", href), slog.String("err", err.Error()))
			return nil, err
		}
		taskList = append(taskList, &Task{URL: taskURL, Index: text})
		slog.InfoContext(ctx, "found task", slog.String("data", firstTD.Data))
	}
	return taskList, nil
}

func fetchSampleIOs(ctx context.Context, task *Task) error {
	taskURL := task.URL
	slog.InfoContext(ctx, "fetching sample IOs", slog.String("url", taskURL.String()))

	resp, err := http.Get(taskURL.String())
	if err != nil {
		slog.ErrorContext(ctx, "failed to send GET request", slog.String("url", taskURL.String()), slog.String("err", err.Error()))
		return err
	}
	defer resp.Body.Close()

	root, err := html.Parse(resp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse HTML", slog.String("url", taskURL.String()), slog.String("err", err.Error()))
		return err
	}

	inouts := map[string]*TaskSampleIO{}

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

		number := strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(
			string([]rune(numberNode.Data)[4:]), "出力例"), "入力例"))
		isInput := strings.HasPrefix(numberNode.Data, "入力例")
		if _, ok := inouts[number]; !ok {
			inouts[number] = &TaskSampleIO{}
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
			inouts[number].Input = strings.Split(strings.TrimSpace(exampleNode.Data), "\n")
		} else {
			inouts[number].Output = strings.Split(strings.TrimSpace(exampleNode.Data), "\n")
		}
		return true
	})

	for num, io := range inouts {
		input, err := os.OpenFile(fmt.Sprintf("sample-%s-input.txt", num), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			slog.ErrorContext(ctx, "failed to open input file", slog.String("file", fmt.Sprintf("sample-%s-input.txt", num)), slog.String("err", err.Error()))
			return err
		}
		defer input.Close()

		_, err = input.WriteString(strings.Join(io.Input, "\n") + "\n")
		if err != nil {
			slog.ErrorContext(ctx, "failed to write input file", slog.String("file", fmt.Sprintf("sample-%s-input.txt", num)), slog.String("err", err.Error()))
			return err
		}

		output, err := os.OpenFile(fmt.Sprintf("sample-%s-output.txt", num), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			slog.ErrorContext(ctx, "failed to open output file", slog.String("file", fmt.Sprintf("sample-%s-output.txt", num)), slog.String("err", err.Error()))
			return err
		}
		defer output.Close()

		_, err = output.WriteString(strings.Join(io.Output, "\n") + "\n")
		if err != nil {
			slog.ErrorContext(ctx, "failed to write output file", slog.String("file", fmt.Sprintf("sample-%s-output.txt", num)), slog.String("err", err.Error()))
			return err
		}
	}
	return nil
}

// 現状 login は不要
// 提出などで必要になる想定
func login(ctx context.Context, u *url.URL) {
	loginURL, err := u.Parse("login")
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse login URL", slog.String("base_url", u.String()), slog.String("err", err.Error()))
		return
	}

	csrfToken, err := findCSRFToken(ctx, loginURL)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find CSRF token", slog.String("url", loginURL.String()), slog.String("err", err.Error()))
		return
	}
	fmt.Println("CSRF Token:", csrfToken)

	// クエリパラメータの設定例

	q := url.Values{}
	q.Add("username", "****") // TODO: 外から受け取る
	q.Add("password", "****") // TODO: 外から受け取る
	q.Add("csrf_token", csrfToken)

	req, err := http.NewRequestWithContext(
		ctx, "POST", loginURL.String(),
		strings.NewReader(q.Encode()),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create login POST request", slog.String("url", loginURL.String()), slog.String("err", err.Error()))
		return
	}
	dump, err := httputil.DumpRequestOut(req, true)
	if err == nil {
		slog.ErrorContext(ctx, "response message", slog.String("response", string(dump)))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send login POST request", slog.String("url", loginURL.String()), slog.String("err", err.Error()))
		return
	}
	defer resp.Body.Close()
	dump, err = httputil.DumpResponse(resp, true)
	if err == nil {
		slog.ErrorContext(ctx, "response message", slog.String("response", string(dump)))
	}
}

func findCSRFToken(ctx context.Context, loginURL *url.URL) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", loginURL.String(), nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create login request", slog.String("url", loginURL.String()), slog.String("err", err.Error()))
		return "", err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to send login request", slog.String("url", loginURL.String()), slog.String("err", err.Error()))
		return "", err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "login request failed", slog.String("url", loginURL.String()), slog.Int("status_code", resp.StatusCode))
		return "", err
	}

	node, err := html.ParseWithOptions(resp.Body)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to parse login response",
			slog.String("url", loginURL.String()),
			slog.String("err", err.Error()),
		)
		return "", err
	}
	fmt.Println(node.Type)
	csrfToken, err := findOneNode(node, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "input" && attrIs(n, "name", "csrf_token") && !attrIs(n, "value", "")
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"failed to find csrf_token input element",
			slog.String("url", loginURL.String()),
			slog.String("err", err.Error()),
		)
		return "", err
	}
	fmt.Println(csrfToken.Data)
	for _, a := range csrfToken.Attr {
		if a.Key == "value" && a.Val != "" {
			return a.Val, nil
		}
	}
	return "", fmt.Errorf("no csrf token")
}

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
