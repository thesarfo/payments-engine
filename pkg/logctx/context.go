package logctx

import "context"

type requestLogFields struct {
	TraceID     string
	ErrorCode   string
	ErrorDetail string
}

type requestLogFieldsKey struct{}

func WithRequestLogFields(ctx context.Context) context.Context {
	if getRequestLogFields(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, requestLogFieldsKey{}, &requestLogFields{})
}

func SetTraceID(ctx context.Context, traceID string) {
	if fields := getRequestLogFields(ctx); fields != nil {
		fields.TraceID = traceID
	}
}

func TraceID(ctx context.Context) string {
	if fields := getRequestLogFields(ctx); fields != nil {
		return fields.TraceID
	}
	return ""
}

func SetError(ctx context.Context, code, detail string) {
	if fields := getRequestLogFields(ctx); fields != nil {
		fields.ErrorCode = code
		fields.ErrorDetail = detail
	}
}

func Error(ctx context.Context) (code, detail string, ok bool) {
	fields := getRequestLogFields(ctx)
	if fields == nil || fields.ErrorCode == "" {
		return "", "", false
	}
	return fields.ErrorCode, fields.ErrorDetail, true
}

func getRequestLogFields(ctx context.Context) *requestLogFields {
	fields, _ := ctx.Value(requestLogFieldsKey{}).(*requestLogFields)
	return fields
}
