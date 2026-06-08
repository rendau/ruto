package middleware

import (
	"net/http"

	"github.com/rendau/ruto/internal/constant"
	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/log"
	"github.com/rendau/ruto/internal/service/gw/service/logmask"
)

func NewRequestLog(
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
	routePath string,
) Middleware {
	if ep.Http.Method != "" {
		routePath = ep.Http.Method + " " + routePath
	}

	lg := ep.Logging
	if lg.EffectiveLevel() == constant.LoggingLevelNone {
		return func(next http.Handler) http.Handler { return next }
	}

	service := log.New(app, ep, routePath, lg)
	sensitive := logmask.BuildSensitiveKeySet(ep)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			service.Serve(func() ([]any, string, error) {
				var reqBody *rw_wrapper.BodyCapture
				if lg.ReqBody && r.Body != nil {
					reqBody = rw_wrapper.NewBodyCapture(lg.ReqBodyLimitOrDefault())
					r.Body = reqBody.WrapReader(r.Body)
				}

				rw := rw_wrapper.New(w)
				if lg.RespBody {
					rw.CaptureBody(lg.RespBodyLimitOrDefault())
				}

				next.ServeHTTP(rw, r)

				// method and path are always logged.
				fields := []any{
					"host", r.Host,
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
				}
				if lg.Headers {
					fields = append(fields, "headers", logmask.MaskValues(r.Header, sensitive))
				}
				if lg.QueryParams {
					fields = append(fields, "query_params", logmask.MaskValues(r.URL.Query(), sensitive))
				}
				if reqBody != nil {
					fields = append(fields, "req_body", reqBody.String())
				}
				if lg.RespBody {
					fields = append(fields, "resp_body", rw.GetCapturedBody())
				}

				return fields, rw.GetStatusCodeStr(), rw.GetStatusCodeErr()
			})
		})
	}
}
