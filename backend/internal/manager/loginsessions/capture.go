package loginsessions

// capture.go: snapshot a Chromium session's cookies + per-origin localStorage
// (sessionStorage intentionally omitted — Playwright's storageState format
// doesn't carry it). The output is a JSON blob in the exact shape that
// `BrowserContext.storageState()` produces and `chromium.newContext()`
// consumes, so an agent can later load it via:
//
//   const ctx = await browser.newContext({ storageState: JSON.parse(env) });

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// StorageState is Playwright's persisted-state shape. Fields kept minimal —
// extra keys (e.g. `version`) are tolerated by Playwright on load.
type StorageState struct {
	Cookies []StorageCookie `json:"cookies"`
	Origins []OriginStorage `json:"origins"`
}

type StorageCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite,omitempty"`
}

type OriginStorage struct {
	Origin       string        `json:"origin"`
	LocalStorage []StorageItem `json:"localStorage"`
}

type StorageItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CaptureResult is what we hand back to the HTTP layer for inclusion in
// the POST /capture response.
type CaptureResult struct {
	SecretName  string `json:"secretName"`
	SizeBytes   int    `json:"sizeBytes"`
	CookieCount int    `json:"cookieCount"`
	OriginCount int    `json:"originCount"`
	JSON        string `json:"-"`
}

// Capture connects to the session's Chromium over CDP, scrapes cookies +
// localStorage per origin, and returns a CaptureResult. The caller is
// responsible for persisting the JSON to a project secret; we keep the
// secret-writing in the HTTP handler so the manager package stays free of
// project-service coupling.
func (m *Manager) Capture(ctx context.Context, sid string) (*CaptureResult, error) {
	sess, ok := m.Get(sid)
	if !ok {
		return nil, errors.New("session not found")
	}

	cdp, err := DialCDP(ctx, sess.DevToolsAddr(), sess.HostHeader())
	if err != nil {
		return nil, fmt.Errorf("connect cdp: %w", err)
	}
	defer cdp.Close()

	targets, err := cdp.GetTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}

	// Browser-level cookies (covers all open tabs in this context).
	cookies, err := getAllCookies(ctx, cdp)
	if err != nil {
		return nil, fmt.Errorf("get cookies: %w", err)
	}

	// Per-page localStorage. We dedupe by origin so a site open in two
	// tabs doesn't get its storage exported twice.
	originsByName := map[string][]StorageItem{}
	for _, t := range targets {
		if t.Type != "page" {
			continue
		}
		sessionID, err := cdp.FindPageSessionID(ctx, t.TargetID)
		if err != nil {
			continue
		}
		origin, items, err := scrapeLocalStorage(ctx, cdp, sessionID)
		if err != nil || origin == "" {
			continue
		}
		// First write wins; we don't merge across tabs because keys
		// could conflict and we don't know which is "newest".
		if _, exists := originsByName[origin]; !exists {
			originsByName[origin] = items
		}
	}

	state := StorageState{
		Cookies: cookies,
		Origins: make([]OriginStorage, 0, len(originsByName)),
	}
	for origin, items := range originsByName {
		state.Origins = append(state.Origins, OriginStorage{Origin: origin, LocalStorage: items})
	}

	buf, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("encode storage state: %w", err)
	}

	return &CaptureResult{
		SecretName:  sess.SecretName,
		SizeBytes:   len(buf),
		CookieCount: len(cookies),
		OriginCount: len(state.Origins),
		JSON:        string(buf),
	}, nil
}

func getAllCookies(ctx context.Context, cdp *CDPClient) ([]StorageCookie, error) {
	// Prefer Storage.getCookies (browser-level, always available); fall
	// back to Network.getAllCookies if the Storage domain isn't exposed
	// on this particular Chromium build.
	raw, err := cdp.Send(ctx, "Storage.getCookies", nil)
	if err != nil {
		raw, err = cdp.Send(ctx, "Network.getAllCookies", nil)
	}
	if err != nil {
		return nil, err
	}
	var out struct {
		Cookies []struct {
			Name     string  `json:"name"`
			Value    string  `json:"value"`
			Domain   string  `json:"domain"`
			Path     string  `json:"path"`
			Expires  float64 `json:"expires"`
			HTTPOnly bool    `json:"httpOnly"`
			Secure   bool    `json:"secure"`
			SameSite string  `json:"sameSite"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	cookies := make([]StorageCookie, 0, len(out.Cookies))
	for _, c := range out.Cookies {
		cookies = append(cookies, StorageCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: normalizeSameSite(c.SameSite),
		})
	}
	return cookies, nil
}

// normalizeSameSite maps CDP's "Strict"/"Lax"/"None"/"" to the Playwright
// shape, which wants "Strict" | "Lax" | "None" — same casing, so this is
// mostly a no-op but we guard against an unexpected value polluting our
// JSON.
func normalizeSameSite(v string) string {
	switch v {
	case "Strict", "Lax", "None":
		return v
	default:
		return ""
	}
}

// scrapeLocalStorage runs a small JS in the page's main world and returns
// its origin + entries. Returns ("", nil, nil) for about:blank and other
// pages that don't have a meaningful origin (where localStorage throws).
func scrapeLocalStorage(ctx context.Context, cdp *CDPClient, sessionID string) (string, []StorageItem, error) {
	expr := `(() => {
		try {
			const out = { origin: location.origin || "", items: [] };
			if (!out.origin || out.origin === "null") return out;
			for (let i = 0; i < localStorage.length; i++) {
				const k = localStorage.key(i);
				if (k === null) continue;
				out.items.push({ name: k, value: localStorage.getItem(k) ?? "" });
			}
			return out;
		} catch (e) { return { origin: "", items: [] }; }
	})()`
	raw, err := cdp.SendOn(ctx, sessionID, "Runtime.evaluate", map[string]any{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  false,
	})
	if err != nil {
		return "", nil, err
	}
	var resp struct {
		Result struct {
			Value struct {
				Origin string        `json:"origin"`
				Items  []StorageItem `json:"items"`
			} `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", nil, err
	}
	if resp.ExceptionDetails != nil {
		return "", nil, nil
	}
	return resp.Result.Value.Origin, resp.Result.Value.Items, nil
}
