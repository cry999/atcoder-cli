package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

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
