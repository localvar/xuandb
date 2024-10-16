package meta

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/localvar/xuandb/pkg/utils"
)

// ErrNoMetaService means there's no available meta serice in the cluster.
var ErrNoMetaService = &utils.StatusError{
	Code: http.StatusServiceUnavailable,
	Msg:  "no meta service available",
}

// sendRequestToLeader sends an HTTP post request to the leader node of the
// meta service.
func sendRequestToLeader(method, pathAndQuery string, data any) error {
	addr := LeaderHTTPAddr()
	if addr == "" {
		return ErrNoMetaService
	}

	var body io.Reader
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return err
		}
		body = bytes.NewReader(d)
	}

	url := "http://" + addr + pathAndQuery
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode < 300 {
		return nil
	}

	return utils.FromHTTPResponse(resp)
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
