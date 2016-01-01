package tmpl

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

// Sql returns a function suitable for use in a template.FuncMap for issuing sql queries on db.
func Sql(db *sql.DB) func(query string, args ...interface{}) (<-chan []interface{}, error) {
	return (&dbhandler{db: db, stmt: make(map[string]*sql.Stmt)}).sql
}

func SqlDebug(db *sql.DB) func(query string, args ...interface{}) (<-chan []interface{}, error) {
	return (&dbhandler{db: db, debug: true, stmt: make(map[string]*sql.Stmt)}).sql
}

type dbhandler struct {
	db    *sql.DB
	debug bool
	sync.Mutex
	stmt map[string]*sql.Stmt
}

func (h *dbhandler) prep(query string) (*sql.Stmt, error) {
	h.Lock()
	defer h.Unlock()
	stmt := h.stmt[query]
	if stmt == nil {
		// unfortunately there's no way to do this before the first execution
		var err error
		stmt, err = h.db.Prepare(query)
		if err != nil {
			return nil, err
		}
		h.stmt[query] = stmt
	}
	return stmt, nil
}

func (h *dbhandler) sql(query string, args ...interface{}) (<-chan []interface{}, error) {
	stmt, err := h.prep(query)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	retn, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	ch := make(chan []interface{})
	go func() {
		if h.debug {
			defer func(start time.Time) { log.Printf("%v %q", time.Now().Sub(start), query) }(time.Now())
		}
		defer close(ch)
		// this would leak a goroutine if the calling template does not
		// complete the iteration over the returned channel, so we guard with a timeout.
		to := time.After(time.Minute)
	L:
		for rows.Next() {
			retv := make([]interface{}, len(retn))
			retvv := make([]interface{}, len(retn))
			for i := range retv {
				retvv[i] = &retv[i]
			}
			if err := rows.Scan(retvv...); err != nil {
				log.Printf("Error on scan: %v Query: %q", err, query)
				break
			}

			for i, v := range retv {
				if vv, ok := v.([]byte); ok {
					retv[i] = string(vv)
				}
			}

			select {
			case ch <- retv:
				// nix
			case <-to:
				log.Printf("Query timed out: %q", query)
				break L
			}
		}
		if err := rows.Close(); err != nil {
			log.Printf("Error on close: %v Query: %q", err, query)
		}
	}()

	return ch, nil
}
