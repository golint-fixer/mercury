package requesttree

import (
	"golang.org/x/net/context"

	terrors "github.com/mondough/typhon/errors"
	"github.com/obeattie/mercury"
)

const (
	parentIdHeader = "Parent-Request-ID"
	reqIdCtxKey    = "Request-ID"
)

type requestTreeMiddleware struct{}

func (m requestTreeMiddleware) ProcessClientRequest(req mercury.Request) mercury.Request {
	if req.Headers()[parentIdHeader] == "" { // Don't overwrite an exiting header
		if parentId, ok := req.Context().Value(reqIdCtxKey).(string); ok && parentId != "" {
			req.SetHeader(parentIdHeader, parentId)
		}
	}
	return req
}

func (m requestTreeMiddleware) ProcessClientResponse(rsp mercury.Response, ctx context.Context) mercury.Response {
	return rsp
}

func (m requestTreeMiddleware) ProcessClientError(err *terrors.Error, ctx context.Context) {}

func (m requestTreeMiddleware) ProcessServerRequest(req mercury.Request) (mercury.Request, mercury.Response) {
	req.SetContext(context.WithValue(req.Context(), reqIdCtxKey, req.Id()))
	if v := req.Headers()[parentIdHeader]; v != "" {
		req.SetContext(context.WithValue(req.Context(), parentIdCtxKey, v))
	}
	return req, nil
}

func (m requestTreeMiddleware) ProcessServerResponse(rsp mercury.Response, ctx context.Context) mercury.Response {
	if v, ok := ctx.Value(parentIdCtxKey).(string); ok && v != "" && rsp != nil {
		rsp.SetHeader(parentIdHeader, v)
	}
	return rsp
}

func Middleware() requestTreeMiddleware {
	return requestTreeMiddleware{}
}
