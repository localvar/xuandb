package meta

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/localvar/xuandb/pkg/xerrors"
)

// ErrMetaServiceUnavailable means there's no available meta serice.
var ErrMetaServiceUnavailable = xerrors.New(http.StatusServiceUnavailable, "meta service is unavailable")

// sendRequestToLeader sends an HTTP post request to the leader node of the
// meta service.
func sendRequestToLeader(method, pathAndQuery string, data any) error {
	addr := LeaderHTTPAddr()
	if addr == "" {
		return ErrMetaServiceUnavailable
	}

	var body io.Reader
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return xerrors.Wrap(err, http.StatusInternalServerError)
		}
		body = bytes.NewReader(d)
	}

	url := "http://" + addr + pathAndQuery
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return xerrors.Wrap(err, http.StatusInternalServerError)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Wrap(err, http.StatusInternalServerError)
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode < 300 {
		return nil
	}

	return xerrors.FromHTTPResponse(resp)
}

func sendPostRequestToLeader(pathAndQuery string, data any) error {
	return sendRequestToLeader(http.MethodPost, pathAndQuery, data)
}

func sendPutRequestToLeader(pathAndQuery string, data any) error {
	return sendRequestToLeader(http.MethodPut, pathAndQuery, data)
}

func sendDeleteRequestToLeader(pathAndQuery string) error {
	return sendRequestToLeader(http.MethodDelete, pathAndQuery, nil)
}
