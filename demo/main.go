// Demo for the template server
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/lvdlvd/go-net-http-tmpl"
)

var (
	port      = flag.String("http", ":6060", "Port to serve http on.")
	templates = flag.String("templates", "./*.html", "Path to dir with template webpages.")
)

func main() {

	h := tmpl.NewHandler(*templates, nil, nil)

	log.Fatal(http.ListenAndServe(*port, tmpl.Gzip(h)))
}
