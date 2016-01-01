// Demo for the template server
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/lvdlvd/go-net-http-tmpl"

	_ "github.com/mattn/go-sqlite3"
)

var (
	port      = flag.String("http", ":6060", "Port to serve http on.")
	templates = flag.String("templates", "./*.html", "Path to dir with template webpages.")
)

func main() {

	os.Remove("./foo.db")

	db, err := sql.Open("sqlite3", "./foo.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	prepareDB(db)

	h := tmpl.NewHandler(*templates, nil, template.FuncMap{
		"sql":   tmpl.SqlDebug(db),
		"group": tmpl.Group,
	})

	log.Fatal(http.ListenAndServe(*port, tmpl.Gzip(h)))
}

func prepareDB(db *sql.DB) {

	sqlStmt := `
	create table foo (id integer not null primary key, name text);
	delete from foo;
	`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("insert into foo(id, name) values(?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	for i := 0; i < 100; i++ {
		_, err = stmt.Exec(i, fmt.Sprintf("こんにちわ世界%03d", i))
		if err != nil {
			log.Fatal(err)
		}
	}
	tx.Commit()

}
