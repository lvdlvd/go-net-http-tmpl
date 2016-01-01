package tmpl

type grouped struct {
	Group []interface{}
	Inner <-chan []interface{}
}

func eq(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// Group is a function suitable for use in a template.FuncMap
// It is meant to be used in combination with the Sql functor.
// It will split the records in a channel returned by the sql query
// into an 'outer' Group and an Inner channel, over which the template
// can peform a nested {{range}}.
func Group(nfields int, all <-chan []interface{}) <-chan grouped {
	outer := make(chan grouped)
	go func() {
		var (
			grp   []interface{}
			inner chan []interface{}
		)
		for rec := range all {
			n := len(rec)
			if n > nfields {
				n = nfields
			}
			fx, vr := rec[:n], rec[n:]
			if !eq(fx, grp) {
				if inner != nil {
					close(inner)
				}
				grp = fx
				inner = make(chan []interface{})
				outer <- grouped{grp, inner}
			}
			inner <- vr
		}
		if inner != nil {
			close(inner)
		}
		close(outer)
	}()
	return outer
}
