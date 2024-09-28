package metaapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/localvar/xuandb/pkg/conf"
)

// LeaderHintHeader is the header name, the meta service set this header with
// the leader address as hint for the clients.
const LeaderHintHeader = "X-Meta-Leader-Hint"

// ErrNoMetaService means there's no available meta serice in the cluster.
var ErrNoMetaService = errors.New("no meta service available")

// buildRequestFunc is a function type, the functions build a request which will
// be sent to server address 'addr'.
type buildRequestFunc func(addr string) (*http.Request, error)

// leaderAddr is the leader address we used to send the last request.
var leaderAddr atomic.Value

// sendRequestToLeader sends a request to the leader of the meta service.
func sendRequestToLeader(buildRequest buildRequestFunc) (resp *http.Response, err error) {
	err = ErrNoMetaService
	addr, nodes, idx := "", conf.Nodes(), 0

	// first we try the leader address we used last time.
	if la := leaderAddr.Load(); la != nil {
		addr = la.(string)
	}

	// tried is the set of addresses we have tried.
	tried := map[string]struct{}{}
	for idx < len(nodes) {
		// if we don't have a valid leader address, get the address of the next
		// node which includes meta service.
		if addr == "" {
			n := nodes[idx]
			idx++
			if n.MetaService == nil {
				continue
			}
			addr = n.HTTPAddr
		}

		// skip addresses which were already tried.
		if _, ok := tried[addr]; ok {
			addr = ""
			continue
		}

		// mark this address as tried.
		tried[addr] = struct{}{}

		// build & send the request.
		var req *http.Request
		req, err = buildRequest(addr)
		if err != nil {
			return nil, err
		}
		resp, err = http.DefaultClient.Do(req)

		// cannot send the request, clear addr and try next.
		if err != nil {
			addr = ""
			continue
		}

		// regard 2xx as success, save addr as current leader address.
		if resp.StatusCode < 300 {
			leaderAddr.Store(addr)
			return resp, nil
		}

		// the body is the error message, but if we failed to read the body,
		// then we have to use the read error.
		if body, e1 := io.ReadAll(resp.Body); e1 != nil {
			err = e1
		} else {
			err = errors.New(string(body))
		}
		resp.Body.Close()

		// the meta service never return 1xx & 3xx, and no need to retry 4xx
		// because they are client side errors.
		if resp.StatusCode < 500 {
			break
		}

		// for 5xx, use the value of the hint header as the next leader address.
		// if the response does not contain this header, 'addr' will be empty.
		addr = resp.Header.Get(LeaderHintHeader)
	}

	return nil, err
}

type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func AddUser(u User) error {
	data, err := json.Marshal(&u)
	if err != nil {
		return err
	}

	fn := func(addr string) (*http.Request, error) {
		url := "http://" + addr + "/meta/user"
		return http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	}
	sendRequestToLeader(fn)
	return nil
}
