package query

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/query/ast"
	"github.com/localvar/xuandb/pkg/query/parser"
	"github.com/localvar/xuandb/pkg/xerrors"
)

type queryResultWriter struct {
	w        http.ResponseWriter
	err      error
	columns  []string
	numRow   int
	numValue int
}

func (qrw *queryResultWriter) SetError(err error) {
	qrw.err = err
}

func (qrw *queryResultWriter) SetColumns(columns ...string) {
	qrw.columns = columns
}

func (qrw *queryResultWriter) AddRow(vals ...ast.FieldValue) error {
	if qrw.err != nil {
		return qrw.err
	}

	if qrw.numRow == 0 {
		_, err := qrw.w.Write([]byte(`{"columns":[`))
		if err != nil {
			qrw.err = err
			return err
		}

		for i, c := range qrw.columns {
			if i > 0 {
				_, err = qrw.w.Write([]byte(`,`))
				if err != nil {
					qrw.err = err
					return err
				}
			}
			_, err = qrw.w.Write(strconv.AppendQuote(nil, c))
			if err != nil {
				qrw.err = err
				return err
			}
		}

		_, err = qrw.w.Write([]byte(`],"values":[`))
		if err != nil {
			qrw.err = err
			return err
		}
	}

	for i, v := range vals {
		var err error
		if i > 0 {
			_, err = qrw.w.Write([]byte(`,`))
		} else if qrw.numRow > 0 {
			_, err = qrw.w.Write([]byte(`,[`))
		} else {
			_, err = qrw.w.Write([]byte(`[`))
		}
		if err != nil {
			qrw.err = err
			return err
		}

		switch v.Type {
		case ast.FieldValueTypeNil:
			qrw.w.Write([]byte(`null`))
		case ast.FieldValueTypeTime:
			qrw.w.Write([]byte(strconv.FormatInt(v.Time.UnixNano(), 10)))
		case ast.FieldValueTypeDuration:
			qrw.w.Write([]byte(strconv.FormatInt(int64(v.Duration), 10)))
		case ast.FieldValueTypeBool:
			qrw.w.Write([]byte(strconv.FormatBool(v.Bool)))
		case ast.FieldValueTypeInt:
			qrw.w.Write([]byte(strconv.FormatInt(v.Int, 10)))
		case ast.FieldValueTypeFloat:
			qrw.w.Write([]byte(strconv.FormatFloat(v.Float, 'g', -1, 64)))
		case ast.FieldValueTypeString:
			qrw.w.Write(strconv.AppendQuote(nil, v.String))
		default:
			panic("unexpected")
		}
	}

	if _, err := qrw.w.Write([]byte(`]`)); err != nil {
		qrw.err = err
		return err
	}

	qrw.numRow++
	return nil
}

func (qrw *queryResultWriter) Flush() error {
	if err := qrw.err; err != nil {
		if se, ok := qrw.err.(*xerrors.StatusError); ok {
			http.Error(qrw.w, se.Msg, se.StatusCode)
		} else {
			http.Error(qrw.w, err.Error(), http.StatusInternalServerError)
		}
		return err
	}

	if qrw.numRow == 0 {
		qrw.w.WriteHeader(http.StatusNoContent)
		return nil
	}

	if _, err := qrw.w.Write([]byte(`]}`)); err != nil {
		qrw.err = err
		if se, ok := qrw.err.(*xerrors.StatusError); ok {
			http.Error(qrw.w, se.Msg, se.StatusCode)
		} else {
			http.Error(qrw.w, err.Error(), http.StatusInternalServerError)
		}
		return err
	}

	return nil
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

	qrw := &queryResultWriter{w: w}
	if err = stmt.Execute(qrw); err == nil {
		err = qrw.Flush()
	}

	if err != nil {
		slog.Error(
			"failed to execute query",
			slog.String("query", q),
			slog.String("error", err.Error()),
		)
	}
}

// StartService starts the query service.
func StartService() error {
	httpserver.HandleFunc("/query", queryHandler)
	return nil
}

// ShutdownService shuts down the query service.
func ShutdownService() {
	slog.Info("query service stopped")
}
