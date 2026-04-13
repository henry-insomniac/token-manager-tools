package logincallback

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const StorageKey = "token-manager-last-login"

type PageData struct {
	Title         string
	Body          string
	ProfileName   string
	Status        string
	StorageKey    string
	RedirectURL   string
	RedirectDelay int
}

func WriteHTML(w http.ResponseWriter, page PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if page.RedirectDelay <= 0 {
		page.RedirectDelay = 1400
		if page.Status == "success" {
			page.RedirectDelay = 900
		}
	}

	payload, _ := json.Marshal(map[string]string{
		"status":      page.Status,
		"title":       page.Title,
		"body":        page.Body,
		"profileName": page.ProfileName,
		"at":          time.Now().Format(time.RFC3339Nano),
	})
	payloadScript := strings.ReplaceAll(string(payload), "</", "<\\/")

	var extraScript strings.Builder
	if strings.TrimSpace(page.StorageKey) != "" {
		fmt.Fprintf(&extraScript, `
  const storageKey = %q;
  try {
    localStorage.setItem(storageKey, JSON.stringify(payload));
  } catch {}
`, page.StorageKey)
	}
	if strings.TrimSpace(page.RedirectURL) != "" {
		fmt.Fprintf(&extraScript, `
  window.setTimeout(() => {
    try {
      window.close();
    } catch {}
    window.location.replace(%q);
  }, %d);
`, page.RedirectURL, page.RedirectDelay)
	} else {
		fmt.Fprintf(&extraScript, `
  window.setTimeout(() => {
    try {
      window.close();
    } catch {}
  }, %d);
`, page.RedirectDelay)
	}

	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    body{margin:0;min-height:100vh;display:grid;place-items:center;background:#151916;color:#e7ece4;font-family:"Avenir Next","PingFang SC","Microsoft YaHei UI",sans-serif}
    main{width:min(520px,calc(100vw - 40px));border:1px solid #354136;background:#20261f;border-radius:28px;padding:34px}
    h1{margin:0 0 12px;font-size:28px}
    p{margin:0;color:#aeb8aa;line-height:1.7}
    a{display:inline-flex;margin-top:16px;color:#dceec7}
  </style>
</head>
<body>
<main>
  <h1>%s</h1>
  <p>%s</p>
  <a href="%s">返回</a>
</main>
<script>
  const payload = %s;
%s
</script>
</body>
</html>`, escapeHTML(page.Title), escapeHTML(page.Title), escapeHTML(page.Body), escapeHTML(firstNonEmpty(page.RedirectURL, "javascript:history.back()")), payloadScript, extraScript.String())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func escapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
