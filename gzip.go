package tmpl

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// AcceptEncGzip checks if the request includes gzip as accept-encoding.
func acceptEncGzip(r *http.Request) bool {
	for _, v := range r.Header["Accept-Encoding"] {
		if strings.Contains(v, "gzip") { // assume nobody would send gzip;q=0
			return true
		}
	}
	return false
}

// A catchWrites intercepts writes to ResponseWriter by feeding them to its own Writer
type catchWrites struct {
	io.Writer
	http.ResponseWriter
}

func (w catchWrites) Write(b []byte) (int, error) { return w.Writer.Write(b) }

// Gzip wraps a handler such that the response will be gzipped if the request specifies gzip
// as an acceptable encoding.  It is not specific to this packages but useful to have around.
func Gzip(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if acceptEncGzip(r) {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			w = catchWrites{Writer: gz, ResponseWriter: w}
		}
		handler.ServeHTTP(w, r)
	})
}
