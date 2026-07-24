package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"buatpostingan/internal/domain/service"
)

const (
	defaultFetchTimeout  = 15 * time.Second
	defaultFetchMaxChars = 20000
	maxFetchMaxChars     = 100000
	maxFetchBodyBytes    = 2 << 20 // 2 MiB
	maxFetchRedirects    = 5
	fetchUserAgent       = "BuatPostingan-web_fetch/1 (+https://github.com/jahrulnr/BuatPostingan)"
)

func (r *Registry) execWebFetch(ctx context.Context, args map[string]any) service.ToolEnvelope {
	rawURL := strings.TrimSpace(asString(args["url"]))
	if rawURL == "" {
		return fetchFail("validation", "url required")
	}
	maxChars := clamp(asInt(args["max_chars"], defaultFetchMaxChars), 500, maxFetchMaxChars)

	u, err := parseFetchURL(rawURL)
	if err != nil {
		return fetchFail("ssrf_blocked", err.Error())
	}
	// With the default SSRF-safe client, reject literal private/metadata IPs early.
	// Injected FetchClient (httptest) may intentionally dial loopback for unit tests.
	if r.fetchClient == nil {
		if ip := net.ParseIP(u.Hostname()); ip != nil && isBlockedIP(ip) {
			return fetchFail("ssrf_blocked", "ip blocked by SSRF policy")
		}
	}

	client := r.fetchClient
	if client == nil {
		client = newSSRFSafeHTTPClient(defaultFetchTimeout)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fetchFail("tool_error", "failed to build request")
	}
	req.Header.Set("User-Agent", fetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,text/markdown;q=0.9,*/*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		msg := "fetch failed"
		code := "tool_error"
		if isSSRFError(err) {
			code = "ssrf_blocked"
			msg = "url blocked by SSRF policy"
		} else if ctx.Err() != nil {
			code = "timeout"
			msg = "fetch timed out"
		}
		return fetchFail(code, msg)
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	if _, err := parseFetchURL(finalURL); err != nil {
		return fetchFail("ssrf_blocked", "redirect target blocked by SSRF policy")
	}

	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if ct != "" && !allowedFetchContentType(ct) {
		return fetchFail("unsupported_content_type", "content type not allowed (text/html or text only)")
	}

	if resp.ContentLength > maxFetchBodyBytes {
		return fetchFail("oversize", "response body exceeds size limit")
	}

	limited := io.LimitReader(resp.Body, maxFetchBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return fetchFail("tool_error", "failed to read response body")
	}
	if int64(len(body)) > maxFetchBodyBytes {
		return fetchFail("oversize", "response body exceeds size limit")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return service.ToolEnvelope{
			OK:   false,
			Tool: "web_fetch",
			Data: map[string]any{
				"url":        rawURL,
				"final_url":  finalURL,
				"status":     resp.StatusCode,
				"content_type": ct,
			},
			Error: map[string]any{
				"code":    "http_error",
				"message": fmt.Sprintf("HTTP %d", resp.StatusCode),
			},
			Meta: map[string]any{
				"truncated":         false,
				"data_is_untrusted": true,
			},
		}
	}

	title, text := extractReadable(body, ct)
	truncated := false
	if len(text) > maxChars {
		text = text[:maxChars]
		truncated = true
	}

	return service.ToolEnvelope{
		OK:   true,
		Tool: "web_fetch",
		Data: map[string]any{
			"url":          rawURL,
			"final_url":    finalURL,
			"status":       resp.StatusCode,
			"content_type": ct,
			"title":        title,
			"text":         text,
		},
		Error: nil,
		Meta: map[string]any{
			"truncated":         truncated,
			"count":             len(text),
			"data_is_untrusted": true,
		},
	}
}

func fetchFail(code, message string) service.ToolEnvelope {
	return service.ToolEnvelope{
		OK:   false,
		Tool: "web_fetch",
		Data: nil,
		Error: map[string]any{
			"code":    code,
			"message": message,
		},
		Meta: map[string]any{
			"truncated":         false,
			"data_is_untrusted": true,
		},
	}
}

func parseFetchURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return nil, fmt.Errorf("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("only http and https are allowed")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url host required")
	}
	if u.User != nil {
		return nil, fmt.Errorf("urls with credentials are not allowed")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("url host required")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || lower == "metadata.google.internal" {
		return nil, fmt.Errorf("host blocked by SSRF policy")
	}
	// Literal private/metadata IPs blocked here; DialContext re-checks DNS results.
	// Injected FetchClient (httptest tests) may dial loopback — skip literal-IP reject then.
	return u, nil
}

func allowedFetchContentType(ct string) bool {
	media := ct
	if i := strings.Index(ct, ";"); i >= 0 {
		media = strings.TrimSpace(ct[:i])
	}
	switch media {
	case "text/html", "application/xhtml+xml", "text/plain", "text/markdown", "text/x-markdown":
		return true
	default:
		return strings.HasPrefix(media, "text/")
	}
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// CGNAT / shared address space (RFC 6598)
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
		// Documentation / benchmark ranges sometimes used as FakeIP
		if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
			return true
		}
	}
	return false
}

func newSSRFSafeHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}
	dialer := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		Proxy: nil, // do not honor HTTP_PROXY for agent fetches (SSRF surface)
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if ip := net.ParseIP(host); ip != nil {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("ssrf_blocked: ip not allowed")
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ipa := range ips {
				if isBlockedIP(ipa.IP) {
					lastErr = fmt.Errorf("ssrf_blocked: ip not allowed")
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("ssrf_blocked: no safe address")
			}
			return nil, lastErr
		},
		DisableKeepAlives: true,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxFetchRedirects {
				return fmt.Errorf("too many redirects")
			}
			if _, err := parseFetchURL(req.URL.String()); err != nil {
				return fmt.Errorf("ssrf_blocked: %w", err)
			}
			return nil
		},
	}
}

func isSSRFError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "ssrf_blocked") || strings.Contains(msg, "ssrf policy")
}
