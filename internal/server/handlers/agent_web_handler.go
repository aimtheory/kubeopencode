// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opencodeapp "github.com/kubeopencode/kubeopencode/internal/opencode-app"
)

var webLog = ctrl.Log.WithName("agent-web")

// openCodeAPIRoutes lists known OpenCode server API route prefixes.
// Requests matching these are proxied to the agent's OpenCode server.
// All other paths are treated as SPA routes and served index.html.
var openCodeAPIRoutes = []string{
	"/global/", "/session/", "/project/", "/file/", "/pty/",
	"/config/", "/provider/", "/mcp/", "/permission/", "/question/",
	"/tui/", "/experimental/", "/auth/", "/instance/",
	"/event", "/path", "/vcs", "/command", "/agent", "/skill",
	"/lsp", "/formatter", "/doc", "/log",
}

// AgentWebHandler serves the OpenCode Web UI for server-mode agents.
//
// In self-hosted mode (OpenCode Web UI built and embedded), it serves static
// assets from the embedded filesystem and only proxies API calls to the agent's
// OpenCode server. This eliminates the dependency on app.opencode.ai.
//
// In fallback mode (no embedded assets), it proxies all requests to the agent's
// OpenCode server, which in turn proxies static assets from app.opencode.ai.
type AgentWebHandler struct {
	defaultClient client.Client
	selfHostedFS  fs.FS // nil if not self-hosted
}

// NewAgentWebHandler creates a new AgentWebHandler.
func NewAgentWebHandler(c client.Client) *AgentWebHandler {
	assetFS := opencodeapp.Assets()
	if assetFS != nil {
		webLog.Info("Self-hosted OpenCode Web UI enabled (embedded assets found)")
	} else {
		webLog.Info("Self-hosted OpenCode Web UI not available, falling back to proxy mode")
	}
	return &AgentWebHandler{defaultClient: c, selfHostedFS: assetFS}
}

// ServeWeb is the catch-all handler for /api/v1/namespaces/{namespace}/agents/{name}/web/*
func (h *AgentWebHandler) ServeWeb(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	agentName := chi.URLParam(r, "name")
	proxyPath := normalizeProxyPath(r)
	proxyBase := fmt.Sprintf("/api/v1/namespaces/%s/agents/%s/web", namespace, agentName)

	// Self-hosted mode: serve static assets locally, proxy only API calls
	if h.selfHostedFS != nil {
		// Root → serve index.html with fetch patch
		if proxyPath == "/" {
			h.serveIndexHTML(w, proxyBase)
			return
		}

		// Static asset → serve from embedded FS
		assetPath := strings.TrimPrefix(proxyPath, "/")
		if _, err := fs.Stat(h.selfHostedFS, assetPath); err == nil {
			// JS files: rewrite hardcoded localhost:4096 URLs so the SDK
			// creates same-origin requests instead of cross-origin ones.
			// This prevents ALPN/protocol negotiation errors when users
			// have system proxies (e.g., Clash, VPN) that interfere with
			// cross-port localhost requests.
			if strings.HasSuffix(assetPath, ".js") {
				h.serveRewrittenJS(w, r, assetPath)
				return
			}
			http.ServeFileFS(w, r, h.selfHostedFS, assetPath)
			return
		}

		// OpenCode API route → proxy to agent server (resolves K8s API only here)
		if isOpenCodeAPIPath(proxyPath) {
			h.proxyToAgent(w, r, proxyPath, proxyBase)
			return
		}

		// SPA fallback (client-side routes like /workspace/session/123)
		h.serveIndexHTML(w, proxyBase)
		return
	}

	// Fallback mode: proxy everything to the OpenCode server
	h.proxyToAgent(w, r, proxyPath, proxyBase)
}

// openCodeDefaultURL is the hardcoded URL that OpenCode's entry.tsx uses when
// it detects localhost. We replace the string literal (including quotes) with
// the JS expression `location.origin` so the SDK creates same-origin requests.
const openCodeDefaultURL = `"http://localhost:4096"`

// serveRewrittenJS reads a JS file from the embedded FS, replaces hardcoded
// localhost:4096 URLs with location.origin, and serves the modified content.
// This ensures the OpenCode SDK creates same-origin requests, avoiding
// cross-origin protocol negotiation issues with system proxies.
func (h *AgentWebHandler) serveRewrittenJS(w http.ResponseWriter, r *http.Request, assetPath string) {
	data, err := fs.ReadFile(h.selfHostedFS, assetPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read JS asset", err.Error())
		return
	}

	content := string(data)
	if strings.Contains(content, openCodeDefaultURL) {
		content = strings.ReplaceAll(content, openCodeDefaultURL, "location.origin")
	}

	body := []byte(content)
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// serveIndexHTML reads index.html from the embedded FS, injects the fetch
// monkey-patch script, sets security headers, and writes the response.
func (h *AgentWebHandler) serveIndexHTML(w http.ResponseWriter, proxyBase string) {
	data, err := fs.ReadFile(h.selfHostedFS, "index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read index.html", err.Error())
		return
	}

	body := []byte(rewriteHTML(string(data), proxyBase))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setSecurityHeaders(w)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// proxyToAgent resolves the agent server URL and creates a reverse proxy.
func (h *AgentWebHandler) proxyToAgent(w http.ResponseWriter, r *http.Request, proxyPath, proxyBase string) {
	namespace := chi.URLParam(r, "namespace")
	agentName := chi.URLParam(r, "name")

	// Detach from chi's timeout context (60s) to support SSE and long-lived
	// proxy responses. The proxy will still terminate when the client
	// disconnects (httputil.ReverseProxy handles write errors).
	ctx := context.WithoutCancel(r.Context())

	// For non-API paths (e.g., static assets in fallback mode), add a timeout
	// to prevent indefinite hangs if the upstream server becomes unresponsive.
	// API paths may serve SSE streams and must remain unbounded.
	if !isOpenCodeAPIPath(proxyPath) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
	}

	k8sClient := clientFromContext(ctx, h.defaultClient)
	serverURL, err := resolveAgentServerURL(ctx, k8sClient, namespace, agentName)
	if err != nil {
		webLog.Error(err, "Failed to resolve agent server URL", "namespace", namespace, "agent", agentName)
		writeError(w, http.StatusBadGateway, "Cannot resolve agent server", err.Error())
		return
	}

	target, err := url.Parse(serverURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Invalid server URL", err.Error())
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = proxyPath
			req.Host = target.Host
			req.Header.Del("Authorization")
			req.Header.Set("Accept-Encoding", "identity")
		},
		ModifyResponse: func(resp *http.Response) error {
			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, "text/html") {
				return nil
			}
			return rewriteHTMLResponse(resp, proxyBase)
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			webLog.Error(err, "Web proxy error", "namespace", namespace, "agent", agentName, "path", proxyPath)
			writeError(w, http.StatusBadGateway, "Web proxy error", err.Error())
		},
	}

	webLog.V(1).Info("Proxying API request", "namespace", namespace, "agent", agentName, "path", proxyPath, "method", r.Method)
	proxy.ServeHTTP(w, r.WithContext(ctx))
}

// rewriteHTML performs HTML transformations: asset path rewriting + fetch monkey-patch injection.
func rewriteHTML(html, proxyBase string) string {
	// Rewrite root-relative asset paths in HTML attributes.
	// NOTE: Naive string replacement may incorrectly rewrite occurrences inside
	// <script> blocks. This is a known limitation; full HTML parsing was deemed
	// unnecessary for the controlled OpenCode Web UI assets.
	html = strings.ReplaceAll(html, `src="/`, `src="`+proxyBase+`/`)
	html = strings.ReplaceAll(html, `href="/`, `href="`+proxyBase+`/`)

	script := buildFetchPatchScript(proxyBase)
	if strings.Contains(html, "</head>") {
		html = strings.Replace(html, "</head>", script+"</head>", 1)
	} else {
		webLog.Info("HTML has no </head> tag, skipping fetch patch injection")
	}

	return html
}

// rewriteHTMLResponse modifies an HTML response in fallback (non-self-hosted) mode.
func rewriteHTMLResponse(resp *http.Response, proxyBase string) error {
	var bodyReader io.ReadCloser
	var err error

	if resp.Header.Get("Content-Encoding") == "gzip" {
		bodyReader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to decompress gzip response: %w", err)
		}
		defer bodyReader.Close()
		resp.Header.Del("Content-Encoding")
	} else {
		bodyReader = resp.Body
	}

	// Limit read to 10 MB to prevent OOM from malicious/misconfigured upstream
	const maxHTMLSize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(bodyReader, maxHTMLSize))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	resp.Body.Close()

	modifiedBody := []byte(rewriteHTML(string(body), proxyBase))
	resp.Body = io.NopCloser(bytes.NewReader(modifiedBody))
	resp.ContentLength = int64(len(modifiedBody))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))

	setResponseSecurityHeaders(resp)

	return nil
}

// webUICSP is the Content-Security-Policy for the OpenCode Web UI.
const webUICSP = "default-src 'self'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'"

// setSecurityHeaders sets CSP and frame headers on a direct response.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", webUICSP)
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
}

// setResponseSecurityHeaders sets CSP and frame headers on a proxied response.
func setResponseSecurityHeaders(resp *http.Response) {
	resp.Header.Set("Content-Security-Policy", webUICSP)
	resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
}

// isOpenCodeAPIPath returns true if the path matches a known OpenCode server API route.
func isOpenCodeAPIPath(path string) bool {
	for _, route := range openCodeAPIRoutes {
		if path == route || strings.HasPrefix(path, route) ||
			(strings.HasSuffix(route, "/") && path == strings.TrimSuffix(route, "/")) {
			return true
		}
	}
	return false
}

// buildFetchPatchScript returns a <script> tag that monkey-patches window.fetch
// to transparently rewrite API calls so they route through the proxy.
//
// Important: The OpenCode SDK wraps fetch with a customFetch that mutates the
// Request object by setting `req.timeout = false` (a Bun-specific property).
// This non-standard mutation causes browsers to fail with ERR_ALPN_NEGOTIATION_FAILED
// when making POST requests through kubectl port-forward.
//
// To work around this, we decompose Request objects into a clean string URL +
// init dict instead of using `new Request(url, oldReq)`, which would inherit
// the polluted state.
func buildFetchPatchScript(proxyBase string) string {
	return fmt.Sprintf(`<script>
(function(){
  var base=%q;
  var origin=location.origin;
  var _fetch=window.fetch;
  function rewriteUrl(url){
    try{
      var u=new URL(url,origin);
      if(u.origin===origin&&!u.pathname.startsWith(base)){
        u.pathname=base+u.pathname;
        return u.toString();
      }
      var h=u.hostname;
      if((h==='localhost'||h==='127.0.0.1'||h==='0.0.0.0')&&u.origin!==origin){
        return origin+base+u.pathname+u.search;
      }
    }catch(e){}
    return url;
  }
  window.fetch=function(input,init){
    if(typeof input==='string'){
      input=rewriteUrl(input);
      return _fetch.call(this,input,init);
    }
    if(input instanceof Request){
      var newUrl=rewriteUrl(input.url);
      if(newUrl!==input.url){
        var req=input;
        var s=req.signal;
        return req.text().then(function(body){
          var o={method:req.method,headers:req.headers,credentials:req.credentials};
          if(s) o.signal=s;
          if(body) o.body=body;
          return _fetch.call(this,newUrl,o);
        }.bind(this));
      }
    }
    return _fetch.call(this,input,init);
  };
})();
</script>`, proxyBase)
}
