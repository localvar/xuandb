package query

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/query/parser"
	"github.com/localvar/xuandb/pkg/xerrors"
)

// result set layout:
//
//	{
//	  "columns": ["col1", "col2", ...],
//	  "values": [ [val1, val2, ...],    ... ],
//	}
//
// TODO: this is a temporary implmentation which will be refactored later.
// the buffer size should be limited and data should be written to temporary
// file when exceeds the limit.
type resultSetWriter struct {
	buf     bytes.Buffer
	err     error
	columns []string
	numRow  int
}

func (rsw *resultSetWriter) SetError(err error) {
	if rsw.err == nil {
		rsw.err = err
	}
}

func (rsw *resultSetWriter) SetColumns(columns ...string) {
	if rsw.err != nil {
		return
	}

	if rsw.columns != nil {
		panic("columns has already been set.")
	}

	rsw.columns = columns

	rsw.buf.WriteString(`{"columns":[`)
	for i, c := range columns {
		if i > 0 {
			rsw.buf.WriteByte(',')
		}
		rsw.buf.Write(strconv.AppendQuote(nil, c))
	}
	rsw.buf.WriteByte(']')
}

func writeValue(w io.Writer, v any) error {
	var err error
	switch t := v.(type) {
	case nil:
		_, err = w.Write([]byte(`null`))
	case int8:
		_, err = w.Write([]byte(strconv.FormatInt(int64(t), 10)))
	case int16:
		_, err = w.Write([]byte(strconv.FormatInt(int64(t), 10)))
	case int32:
		_, err = w.Write([]byte(strconv.FormatInt(int64(t), 10)))
	case int64:
		_, err = w.Write([]byte(strconv.FormatInt(t, 10)))
	case int:
		_, err = w.Write([]byte(strconv.FormatInt(int64(t), 10)))
	case uint8:
		_, err = w.Write([]byte(strconv.FormatUint(uint64(t), 10)))
	case uint16:
		_, err = w.Write([]byte(strconv.FormatUint(uint64(t), 10)))
	case uint32:
		_, err = w.Write([]byte(strconv.FormatUint(uint64(t), 10)))
	case uint64:
		_, err = w.Write([]byte(strconv.FormatUint(t, 10)))
	case uint:
		_, err = w.Write([]byte(strconv.FormatUint(uint64(t), 10)))
	case float32:
		_, err = w.Write([]byte(strconv.FormatFloat(float64(t), 'g', -1, 32)))
	case float64:
		_, err = w.Write([]byte(strconv.FormatFloat(t, 'g', -1, 64)))
	case string:
		_, err = w.Write(strconv.AppendQuote(nil, t))
	case time.Time:
		_, err = w.Write([]byte(strconv.FormatInt(t.UnixNano(), 10)))
	case time.Duration:
		_, err = w.Write([]byte(strconv.FormatInt(t.Nanoseconds(), 10)))
	case bool:
		_, err = w.Write([]byte(strconv.FormatBool(t)))
	default:
		panic("unexpected")
	}
	return err
}

func (rsw *resultSetWriter) AddRow(vals ...any) error {
	if rsw.err != nil {
		return rsw.err
	}

	if rsw.columns == nil {
		panic("columns must be set before add rows.")
	}

	if len(vals) != len(rsw.columns) {
		panic("column count mismatch.")
	}

	var err error
	defer func() {
		if err != nil {
			rsw.SetError(err)
		}
	}()

	if rsw.numRow == 0 {
		_, err = rsw.buf.WriteString(`,"values":[[`)
	} else {
		_, err = rsw.buf.WriteString(`,[`)
	}
	if err != nil {
		return err
	}

	for i, v := range vals {
		if i > 0 {
			if err = rsw.buf.WriteByte(','); err != nil {
				return err
			}
		}
		if err = writeValue(&rsw.buf, v); err != nil {
			return err
		}
	}

	if err = rsw.buf.WriteByte(']'); err != nil {
		return err
	}

	rsw.numRow++
	return nil
}

func (rsw *resultSetWriter) Flush(w http.ResponseWriter) error {
	if rsw.err != nil {
		return rsw.err
	}

	if rsw.columns == nil {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	var err error
	if rsw.numRow > 0 {
		_, err = rsw.buf.WriteString(`]}`)
	} else {
		err = rsw.buf.WriteByte('}')
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	// we can do nothing to this error because data may already been written
	// to [w]
	_, err = w.Write(rsw.buf.Bytes())
	return err
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	q := r.FormValue("q")
	if q == "" {
		http.Error(w, "query statement is required", http.StatusBadRequest)
		return
	}

	stmt, err := parser.Parse(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("query received", slog.String("query", q))
	/*
		if db := r.FormValue("db"); db != "" {
			stmt.BindDatabase(db)
		}
	*/

	name, pwd, _ := r.BasicAuth()
	if err := stmt.Auth(name, pwd); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	rsw := &resultSetWriter{}
	if err := stmt.Execute(rsw); err != nil {
		if se, ok := err.(*xerrors.StatusError); ok {
			http.Error(w, se.Msg, se.StatusCode)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := rsw.Flush(w); err != nil {
		slog.Error(
			"failed to flush result set",
			slog.String("query", q),
			slog.String("error", err.Error()),
		)
	}
}

// StartService starts the query service.
func StartService() error {
	httpserver.HandleFunc("/query", queryHandler)
	slog.Info("query service started")
	return nil
}

// ShutdownService shuts down the query service.
func ShutdownService() {
	slog.Info("query service stopped")
}
