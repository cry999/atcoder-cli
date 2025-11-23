package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cry999/atcoder-cli/config"
	"github.com/cry999/atcoder-cli/contests"
	"github.com/cry999/atcoder-cli/contests/adt"
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

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	config, err := config.LoadConfig(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load config", slog.String("err", err.Error()))
		return
	}

	var (
		dumpConfig = flag.Bool("dump-config", false, "Dump loaded config and exit")
		adtLevel   = flag.String("adt-level", string(config.ADT.DefaultLevel), "Default level for ADT problems (easy, medium, hard, all)")
	)
	flag.Parse()

	if *dumpConfig {
		if err := config.Dump(os.Stdout); err != nil {
			slog.ErrorContext(ctx, "failed to dump config", slog.String("err", err.Error()))
		}
		return
	}

	contestFamily := flag.Arg(0)
	if contestFamily == "" {
		slog.ErrorContext(ctx, "contest family argument is required")
		return
	}

	var family contests.Family
	switch contestFamily {
	// AtCoder Daily Training
	case "adt":
		family, err = adt.New(flag.Arg(1), flag.Arg(2), *adtLevel)
		if err != nil {
			slog.ErrorContext(
				ctx, "failed to parse ADT family",
				slog.String("date", flag.Arg(1)),
				slog.String("time", flag.Arg(2)),
				slog.String("err", err.Error()),
			)
			return
		}
	default:
		slog.ErrorContext(ctx, "unknown contest type", slog.String("contest", contestFamily))
		return
	}

	baseDir := family.BaseDir(config.WorkDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		slog.ErrorContext(ctx, "failed to create working directory", slog.String("dir", baseDir), slog.String("err", err.Error()))
		return
	}
	if err := os.Chdir(baseDir); err != nil {
		slog.ErrorContext(ctx, "failed to change working directory", slog.String("dir", baseDir), slog.String("err", err.Error()))
		return
	}

	taskListURL := &url.URL{
		Scheme: "https",
		Host:   DOMAIN,
		Path:   path.Join("contests", family.ContestName(), "tasks"),
	}

	tasks, err := fetchTaskList(ctx, taskListURL)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch task list", slog.String("err", err.Error()))
		return
	}
	for _, task := range tasks {
		if err := os.Mkdir(task.Index, 0755); err != nil && !os.IsExist(err) {
			slog.ErrorContext(ctx, "failed to create task directory", slog.String("dir", task.Index), slog.String("err", err.Error()))
			return
		}
		err = fetchSampleIOs(ctx, task)
		if err != nil {
			slog.ErrorContext(ctx, "failed to fetch sample IOs", slog.String("err", err.Error()))
			return
		}
		for i, io := range task.SampleIOs {
			if err := os.WriteFile(filepath.Join(task.Index, fmt.Sprintf("input-%02d.txt", i)), []byte(strings.Join(io.Input, "\n")+"\n"), 0644); err != nil {
				slog.ErrorContext(ctx, "failed to write sample input file", slog.String("file", filepath.Join(task.Index, fmt.Sprintf("input-%s.txt", task.Index))), slog.String("err", err.Error()))
			}
			if err := os.WriteFile(filepath.Join(task.Index, fmt.Sprintf("output-%02d.txt", i)), []byte(strings.Join(io.Output, "\n")+"\n"), 0644); err != nil {
				slog.ErrorContext(ctx, "failed to write sample output file", slog.String("file", filepath.Join(task.Index, fmt.Sprintf("output-%s.txt", task.Index))), slog.String("err", err.Error()))
			}
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
				slog.ErrorContext(ctx, "failed to parse example number", slog.String("data", numberNode.Data), slog.String("err", err.Error()))
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
