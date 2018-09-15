package hnyaenethttp

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"runtime"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/google/uuid"
	"github.com/honeycombio/beeline-go/trace"
	"github.com/honeycombio/beeline-go/wrappers/common"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

const eventURLBase = "https://api.honeycomb.io/1/events/%v"

const defaultWriteKey = "sample-writekey"
const defaultDataset = "honeycomb-sample"
const defaultServiceName = "service"

// Config ...
type Config struct {
	WriteKey    string
	Dataset     string
	ServiceName string
}

var config Config

// Init ...
func Init(conf Config) {
	c := Config{
		WriteKey:    defaultWriteKey,
		Dataset:     defaultDataset,
		ServiceName: defaultServiceName,
	}

	if conf.WriteKey != "" {
		c.WriteKey = conf.WriteKey
	}
	if conf.Dataset != "" {
		c.Dataset = conf.Dataset
	}
	if conf.ServiceName != "" {
		c.ServiceName = conf.ServiceName
	}

	config = c
	return
}

// WrapHandlerFunc ...
func WrapHandlerFunc(hf func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	handlerFuncName := runtime.FuncForPC(reflect.ValueOf(hf).Pointer()).Name()

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := appengine.NewContext(r)
		start := time.Now()

		tr := trace.GetTraceFromContext(ctx)
		if tr == nil {
			x, y := trace.NewTrace(ctx, "")
			tr = y
			// put the event on the context for everybody downstream to use
			r = r.WithContext(x)
		}

		span := tr.GetRootSpan()

		span.AddField("service_name", config.ServiceName)

		span.AddField("appengine.datacenter", appengine.Datacenter(ctx))
		span.AddField("appengine.module_name", appengine.ModuleName(ctx))
		span.AddField("appengine.requrest_id", appengine.RequestID(ctx))
		span.AddField("appengine.version_id", appengine.VersionID(ctx))

		AddRequestProps(r, span)

		wrappedWriter := common.NewResponseWriter(w)
		//wrappedWriter := NewResponseWriter(w)

		if handlerFuncName != "" {
			span.AddField("handler_func_name", handlerFuncName)
			span.AddField("name", handlerFuncName)
		}
		hf(wrappedWriter, r)
		if wrappedWriter.Status == 0 {
			wrappedWriter.Status = 200
		}
		span.AddField("response.status_code", wrappedWriter.Status)
		span.AddField("duration_ms", float64(time.Since(start))/float64(time.Millisecond))
		err := sendEvent(ctx, span)
		if err != nil {
			log.Debugf(ctx, "error sending event to honeycomb: %v", err)
		}
	}
}

func sendEvent(ctx context.Context, span *trace.Span) error {
	// httpClient := urlfetch.Client(ctx)

	data, err := json.Marshal(span)
	log.Debugf(ctx, "got data: %v", string(data))
	if err != nil {
		return err
	}

	// buf := bytes.NewBuffer(data)
	// req, err := http.NewRequest("POST", fmt.Sprintf(eventURLBase, config.Dataset), buf)
	// req.Header.Set("X-Honeycomb-Team", config.WriteKey)
	// req.Header.Set("X-Honeycomb-Event-Time", time.Now().Format(time.RFC3339))

	// resp, err := httpClient.Do(req)
	// if err != nil {
	// 	return err
	// }

	// if resp.StatusCode != http.StatusOK {
	// 	x, err := ioutil.ReadAll(resp.Body)
	// 	if err != nil {
	// 		return fmt.Errorf("unable to send event, error getting message from response: %v", err)
	// 	}
	// 	_ = resp.Body.Close()
	// 	return fmt.Errorf("unable to send event: %v", x)
	// }
	return nil
}

// everything below here is copied from beeline-go/internal

// ResponseWriter ...
type ResponseWriter struct {
	http.ResponseWriter
	Status int
}

// NewResponseWriter ...
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: httpsnoop.Wrap(w, httpsnoop.Hooks{}),
	}
}

// WriteHeader ...
func (h *ResponseWriter) WriteHeader(statusCode int) {
	h.Status = statusCode
	h.ResponseWriter.WriteHeader(statusCode)
}

// AddRequestProps ...
func AddRequestProps(req *http.Request, ev *trace.Span) {
	// identify the type of event
	ev.AddField("meta.type", "http")
	// Add a variety of details about the HTTP request, such as user agent
	// and method, to any created libhoney event.
	ev.AddField("request.method", req.Method)
	ev.AddField("request.path", req.URL.Path)
	ev.AddField("request.host", req.URL.Host)
	ev.AddField("request.http_version", req.Proto)
	ev.AddField("request.content_length", req.ContentLength)
	ev.AddField("request.remote_addr", req.RemoteAddr)
	ev.AddField("request.header.user_agent", req.UserAgent())

	id, _ := uuid.NewRandom()
	ev.AddField("trace.trace_id", id.String())
	// add a span ID
	id, _ = uuid.NewRandom()
	ev.AddField("trace.span_id", id.String())
}
