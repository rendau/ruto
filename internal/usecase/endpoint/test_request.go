package endpoint

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	"github.com/rendau/ruto/internal/errs"
)

const testRequestMaxBodySize = 1 << 20 // 1 MiB

// pathParamPattern matches single-brace path tokens like {id}. It deliberately
// does not match double-brace {{var}} interpolation tokens.
var pathParamPattern = regexp.MustCompile(`\{([^/{}]+)}`)

type TestRequestResult struct {
	RequestURL    string
	RequestMethod string
	StatusCode    int
	Headers       varsModel.Vars
	Body          string
	DurationMs    int64
	Error         string
}

// TestRequest sends an HTTP request to the endpoint's resolved backend
// (App.Backend.Url + resolved path, with inherited+interpolated headers and
// query params) and returns the response details. Transport failures are
// reported via the result's Error field, not as a gRPC error.
func (u *Usecase) TestRequest(
	ctx context.Context,
	id string,
	pathParams varsModel.Vars,
	queryParams varsModel.Vars,
	body string,
) (*TestRequestResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	epObj, appObj, err := u.resolveInherited(ctx, id, nil, true)
	if err != nil {
		return nil, err
	}

	if epObj.Type == model.TypeGRPC {
		return nil, errs.ErrFull{Err: errs.InvalidRequest, Desc: "grpc endpoints are not testable"}
	}

	base, err := url.Parse(appObj.Backend.Url)
	if err != nil {
		return nil, errs.ErrFull{Err: errs.InvalidConfig, Desc: "invalid backend url: " + err.Error()}
	}
	if base.Host == "" {
		return nil, errs.ErrFull{Err: errs.InvalidConfig, Desc: "app backend url is empty"}
	}

	// The gateway strips app.PathPrefix before proxying, so the backend path is
	// the endpoint's custom_path (if set) or its http.path - never the prefix.
	pathTemplate := epObj.Backend.CustomPath
	if pathTemplate == "" {
		pathTemplate = epObj.Http.Path
	}
	target := base.JoinPath(substitutePathParams(pathTemplate, pathParams))

	// Query params: resolved backend params first, then user overrides win.
	q := target.Query()
	for k, v := range lo.Assign(epObj.Backend.QueryParams, queryParams) {
		q.Set(k, v)
	}
	target.RawQuery = q.Encode()

	method := epObj.Http.Method
	if method == "*" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	for k, v := range epObj.Backend.Headers {
		req.Header.Set(k, v)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	result := &TestRequestResult{
		RequestURL:    target.String(),
		RequestMethod: method,
	}

	startedAt := time.Now()
	resp, err := u.httpClient.Do(req)
	result.DurationMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, testRequestMaxBodySize))
	if err != nil {
		result.Error = "read response body: " + err.Error()
	}

	result.StatusCode = resp.StatusCode
	result.Body = string(respBody)
	result.Headers = lo.MapValues(resp.Header, func(values []string, _ string) string {
		return strings.Join(values, ", ")
	})

	return result, nil
}

// substitutePathParams replaces {name} tokens with their (path-escaped) value.
// Tokens without a provided value are left untouched so a forgotten fill stays
// visible in the resulting URL.
func substitutePathParams(template string, params varsModel.Vars) string {
	if len(params) == 0 {
		return template
	}
	return pathParamPattern.ReplaceAllStringFunc(template, func(token string) string {
		name := token[1 : len(token)-1]
		if v, ok := params[name]; ok {
			return url.PathEscape(v)
		}
		return token
	})
}
