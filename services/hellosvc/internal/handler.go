package internal

import (
	"net/http"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation of HelloHandler
}

func HelloErrorHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation of HelloErrorHandler
}

func HelloKerrorHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation of HelloKerrorHandler
}

func HelloPanicHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "hello_panic", func() {
		kcommon.HelloWithPanic()
	}, "")
}

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/hello", HelloHandler)
	mux.HandleFunc("/api/test_error", HelloErrorHandler)
	mux.HandleFunc("/api/test_kerror", HelloKerrorHandler)
	mux.HandleFunc("/api/test_panic", HelloPanicHandler)
}
