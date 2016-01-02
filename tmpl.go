package tmpl

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// ArgGetter is the type of function that distills the template arguments from a request.
type ArgGetter func(*http.Request) (interface{}, error)

type handler struct {
	pattern string
	getArgs ArgGetter
	fm      template.FuncMap

	sync.Mutex
	lastParsed time.Time
	tmpl       *template.Template
	err        error
}

func (th *handler) recompileIfOlderThan(t time.Time) {
	th.Lock()
	defer th.Unlock()
	if th.lastParsed.After(t) {
		return
	}
	th.lastParsed = time.Now()
	th.tmpl, th.err = template.New("/").Funcs(th.fm).ParseGlob(th.pattern)
	if th.err != nil {
		log.Printf("Compiling templates %q: %v", th.pattern, th.err)
	} else {
		log.Printf("Compiled templates %q: %s", th.pattern, names(th.tmpl))
	}
}

func names(t *template.Template) []string {
	var r []string
	for _, v := range t.Templates() {
		r = append(r, v.Name())
	}
	return r
}

// NewHandler constructs a http.Handler that serves the html templates named by the files in the glob pattern.
//
// Ag must be a function that returns the argument object to template.Execute given a request,
// when nil, the handler will use the GetArgs function from this package.
// Beware that an error returned by ag will be rendered in the 400 response,
// so be sure not to leak sensitive state.
//
// Fm may contain extra functions for use in the templates.
// See https://golang.org/pkg/text/template/#Template.Funcs for more details.
func NewHandler(glob string, ag ArgGetter, fm template.FuncMap) http.Handler {
	if ag == nil {
		ag = GetArgs
	}
	th := &handler{
		pattern: glob,
		getArgs: ag,
		fm:      fm,
	}
	th.recompileIfOlderThan(time.Time{})
	return th
}

// GetArgs is the default function used by NewHandler.
// It constructs a map[string]interface{} with elements from the following sources:
//
// - If the request method is POST or PUT and the content type is application/json
// it will try to parse up to 64kb of json into an object, the elements of which
// become elements of the result. It is not an error if the body is empty.
//
// - Then it will call r.ParseForm to get all GET and POST form parameters,
// which are all of type []string.
//
// - Finally it will copy all gorilla mux.Vars from the request to the result object.
//
// In this list, later values overwrite the earlier ones, so a json object
// element will be overwritten by a mux.Var of the same name.
func GetArgs(r *http.Request) (interface{}, error) {
	args := make(map[string]interface{})

	if r.Method == "POST" || r.Method == "PUT" {
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		ct, _, _ = mime.ParseMediaType(ct)
		if ct == "application/json" {
			defer r.Body.Close()
			err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&args)
			if err != nil && err != io.EOF {
				return nil, err
			}
		}
	}

	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	for k, v := range r.Form { // note: these are []string!
		args[k] = v
	}

	for k, v := range mux.Vars(r) {
		args[k] = v
	}

	return args, nil
}

func lastModified(glob string) (time.Time, error) {
	var t time.Time
	fn, err := filepath.Glob(glob)
	if err != nil {
		return t, err
	}
	for _, v := range fn {
		fi, err := os.Stat(v)
		if err != nil {
			return t, err
		}
		if fi.ModTime().After(t) {
			t = fi.ModTime()
		}
	}
	return t, nil
}

var index = template.Must(template.New("").Parse(`<!DOCTYPE HTML PUBLIC "-//IETF//DTD HTML 2.0//EN">
<html>
<head>
	<meta charset="utf-8">
	<title>Templates</title>
</head>
<body>
<ul>
{{range .}}<li><a href="{{.}}">{{.}}</a></li>
{{else}}<li>No templates found!</li>
{{end}}</ul>
</body>
`))

// ServeHTTP serves the template named by the last component of r.URL.Path.
//
// If the path is '/', it executes the template named 'index'.  If there is no such
// template, one will be syntesized from the names of the defined templates.
// It is the responsability of the caller to ensure the permissions have been checked!
// The advantage of having the partial templates accessible directly is that with
// JQuery it is very easy to substitute part of a page dynamically from an XttpRequest.
func (th *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lastMod, err := lastModified(th.pattern)
	if err != nil {
		log.Println("Stat templates: ", err)
		http.Error(w, "Missing templates?", http.StatusInternalServerError)
		return
	}
	th.recompileIfOlderThan(lastMod)
	if th.err != nil || th.tmpl == nil {
		http.Error(w, "Miscompiled templates.", http.StatusInternalServerError)
		return
	}

	name := path.Base(r.URL.Path)
	if name == "/" {
		name = "index"
	}

	t := th.tmpl.Lookup(name)
	if t == nil {
		if name != "index" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := index.Execute(w, names(th.tmpl)); err != nil {
			log.Printf("Error rendering index template: %v", err)
		}
		return
	}

	args, err := th.getArgs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ww := countWriter{w: w}
	if err := t.Execute(&ww, args); err != nil {
		log.Printf("Executing template %q: %v", name, err)
		if ww.N == 0 {
			http.Error(w, "Error rendering template.", http.StatusInternalServerError)
			return
		}
	}
}

// a countwriter wraps any other writer and tracks how many bytes are written to it.
type countWriter struct {
	w io.Writer
	N int
}

func (w *countWriter) Write(b []byte) (int, error) {
	n, err := w.w.Write(b)
	w.N += n
	return n, err
}
