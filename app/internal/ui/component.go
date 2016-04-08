package ui

import (
	"encoding/json"
	"net/http"
	"time"

	"gopkg.in/inconshreveable/log15.v2"

	"golang.org/x/net/context"
	"sourcegraph.com/sourcegraph/sourcegraph/app/jscontext"
)

// RenderResult is the "HTTP response"-like data returned by the
// JavaScript server-side rendering operation.
type RenderResult struct {
	Body             string          // HTTP response body
	Error            string          // internal error message (should only be shown to admins, may contain secret info)
	Stores           json.RawMessage // contents of stores after prerendering (for client bootstrapping)
	StatusCode       int             // HTTP status code for response
	ContentType      string          // HTTP Content-Type response header
	RedirectLocation string          // HTTP Location header
}

type renderState struct {
	JSContext  jscontext.JSContext    `json:"jsContext"`
	Location   string                 `json:"location"`
	ExtraProps map[string]interface{} `json:"extraProps"`
}

// RenderRouter calls into JavaScript (using jsserver) to render the
// page for the given HTTP request.
var RenderRouter = func(ctx context.Context, req *http.Request, extraProps map[string]interface{}) (*RenderResult, error) {
	jsctx, err := jscontext.NewJSContextFromRequest(req)
	if err != nil {
		return nil, err
	}

	return renderRouterState(ctx, &renderState{
		JSContext:  jsctx,
		Location:   req.URL.String(),
		ExtraProps: extraProps,
	})
}

func renderRouterState(ctx context.Context, state *renderState) (*RenderResult, error) {
	if ctx == nil || !shouldPrerenderReact(ctx) {
		return nil, nil
	}

	arg, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()

	data, err := renderReactComponent(ctx, arg)
	if err != nil {
		log15.Warn("Error rendering React component on the server (falling back to client-side rendering)", "err", err, "arg", truncateArg(arg))
		return nil, nil
	}

	var res *RenderResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func truncateArg(arg []byte) string {
	if max := 300; len(arg) > max {
		arg = arg[:max]
	}
	return string(arg)
}
