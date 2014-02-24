package etcd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Errors introduced by handling requests
var (
	ErrRequestCancelled = errors.New("sending request is cancelled")
)

// getCancelable issues a cancelable GET request
func (c *Client) getCancelable(key string, options options,
	cancel <-chan bool) (*RawResponse, error) {
	logger.Debugf("get %s [%s]", key, c.cluster.Leader)
	p := keyToPath(key)

	// If consistency level is set to STRONG, append
	// the `consistent` query string.
	if c.config.Consistency == STRONG_CONSISTENCY {
		options["consistent"] = true
	}

	str, err := options.toParameters(VALID_GET_OPTIONS)
	if err != nil {
		return nil, err
	}
	p += str

	resp, err := c.sendRequest("GET", p, nil, cancel)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// get issues a GET request
func (c *Client) get(key string, options options) (*RawResponse, error) {
	return c.getCancelable(key, options, nil)
}

// put issues a PUT request
func (c *Client) put(key string, value string, ttl uint64,
	options options) (*RawResponse, error) {

	logger.Debugf("put %s, %s, ttl: %d, [%s]", key, value, ttl, c.cluster.Leader)
	p := keyToPath(key)

	str, err := options.toParameters(VALID_PUT_OPTIONS)
	if err != nil {
		return nil, err
	}
	p += str

	resp, err := c.sendRequest("PUT", p, buildValues(value, ttl), nil)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// post issues a POST request
func (c *Client) post(key string, value string, ttl uint64) (*RawResponse, error) {
	logger.Debugf("post %s, %s, ttl: %d, [%s]", key, value, ttl, c.cluster.Leader)
	p := keyToPath(key)

	resp, err := c.sendRequest("POST", p, buildValues(value, ttl), nil)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// delete issues a DELETE request
func (c *Client) delete(key string, options options) (*RawResponse, error) {
	logger.Debugf("delete %s [%s]", key, c.cluster.Leader)
	p := keyToPath(key)

	str, err := options.toParameters(VALID_DELETE_OPTIONS)
	if err != nil {
		return nil, err
	}
	p += str

	resp, err := c.sendRequest("DELETE", p, nil, nil)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// sendRequest sends a HTTP request and returns a Response as defined by etcd
func (c *Client) sendRequest(method string, relativePath string,
	values url.Values, cancel <-chan bool) (*RawResponse, error) {

	var req *http.Request
	var resp *http.Response
	var httpPath string
	var err error
	var respBody []byte

	reqs := make([]http.Request, 0)
	resps := make([]http.Response, 0)

	checkRetry := c.CheckRetry
	if checkRetry == nil {
		checkRetry = DefaultCheckRetry
	}

	cancelled := false

	if cancel != nil {
		cancelRoutine := make(chan bool)
		defer close(cancelRoutine)

		go func() {
			select {
			case <-cancel:
				cancelled = true
				logger.Debug("send.request is cancelled")
				c.httpClient.Transport.(*http.Transport).CancelRequest(req)
			case <-cancelRoutine:
				return
			}

			// Repeat canceling request until this thread is stopped
			// because we have no idea about whether it succeeds.
			for {
				select {
				case <-time.After(100*time.Millisecond):
					c.httpClient.Transport.(*http.Transport).CancelRequest(req)
				case <-cancelRoutine:
					return
				}
			}
		}()
	}

	// if we connect to a follower, we will retry until we find a leader
	for attempt := 0; ; attempt++ {
		if cancelled {
			return nil, ErrRequestCancelled
		}

		logger.Debug("begin attempt", attempt, "for", relativePath)

		if method == "GET" && c.config.Consistency == WEAK_CONSISTENCY {
			// If it's a GET and consistency level is set to WEAK,
			// then use a random machine.
			httpPath = c.getHttpPath(true, relativePath)
		} else {
			// Else use the leader.
			httpPath = c.getHttpPath(false, relativePath)
		}

		// Return a cURL command if curlChan is set
		if c.cURLch != nil {
			command := fmt.Sprintf("curl -X %s %s", method, httpPath)
			for key, value := range values {
				command += fmt.Sprintf(" -d %s=%s", key, value[0])
			}
			c.sendCURL(command)
		}

		logger.Debug("send.request.to", httpPath, "| method", method)

		if values == nil {
			req, _ = http.NewRequest(method, httpPath, nil)
		} else {
			req, _ = http.NewRequest(method, httpPath,
				strings.NewReader(values.Encode()))

			req.Header.Set("Content-Type",
				"application/x-www-form-urlencoded; param=value")
		}

		resp, err = c.httpClient.Do(req)
		// If the request was cancelled, return ErrRequestCancelled directly
		if cancelled {
			return nil, ErrRequestCancelled
		}

		reqs = append(reqs, *req)

		// network error, change a machine!
		if err != nil {
			logger.Debug("network error:", err.Error())
			resps = append(resps, http.Response{})
			if checkErr := checkRetry(c.cluster, reqs, resps, err); checkErr != nil {
				return nil, checkErr
			}

			c.cluster.switchLeader(attempt % len(c.cluster.Machines))
			continue
		}

		// if there is no error, it should receive response
		resps = append(resps, *resp)
		defer resp.Body.Close()
		logger.Debug("recv.response.from", httpPath)

		if validHttpStatusCode[resp.StatusCode] {
			// try to read byte code and break the loop
			respBody, err = ioutil.ReadAll(resp.Body)
			if err == nil {
				logger.Debug("recv.success.", httpPath)
				break
			}
		}

		// if resp is TemporaryRedirect, set the new leader and retry
		if resp.StatusCode == http.StatusTemporaryRedirect {
			u, err := resp.Location()

			if err != nil {
				logger.Warning(err)
			} else {
				// Update cluster leader based on redirect location
				// because it should point to the leader address
				c.cluster.updateLeaderFromURL(u)
				logger.Debug("recv.response.relocate", u.String())
			}
			continue
		}

		if checkErr := checkRetry(c.cluster, reqs, resps,
			errors.New("Unexpected HTTP status code")); checkErr != nil {
			return nil, checkErr
		}
	}

	r := &RawResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Header:     resp.Header,
	}

	return r, nil
}

// DefaultCheckRetry checks retry cases
// If it has retried 2 * machine number, stop to retry it anymore
// If resp is nil, sleep for 200ms
// If status code is InternalServerError, sleep for 200ms.
func DefaultCheckRetry(cluster *Cluster, reqs []http.Request,
	resps []http.Response, err error) error {

	if len(reqs) >= 2*len(cluster.Machines) {
		return newError(ErrCodeEtcdNotReachable,
			"Tried to connect to each peer twice and failed", 0)
	}

	resp := &resps[len(resps)-1]

	if resp == nil {
		time.Sleep(time.Millisecond * 200)
		return nil
	}

	code := resp.StatusCode
	if code == http.StatusInternalServerError {
		time.Sleep(time.Millisecond * 200)

	}

	logger.Warning("bad response status code", code)
	return nil
}

func (c *Client) getHttpPath(random bool, s ...string) string {
	var machine string
	if random {
		machine = c.cluster.Machines[rand.Intn(len(c.cluster.Machines))]
	} else {
		machine = c.cluster.Leader
	}

	fullPath := machine + "/" + version
	for _, seg := range s {
		fullPath = fullPath + "/" + seg
	}

	return fullPath
}

// buildValues builds a url.Values map according to the given value and ttl
func buildValues(value string, ttl uint64) url.Values {
	v := url.Values{}

	if value != "" {
		v.Set("value", value)
	}

	if ttl > 0 {
		v.Set("ttl", fmt.Sprintf("%v", ttl))
	}

	return v
}

// convert key string to http path exclude version
// for example: key[foo] -> path[keys/foo]
// key[/] -> path[keys/]
func keyToPath(key string) string {
	p := path.Join("keys", key)

	// corner case: if key is "/" or "//" ect
	// path join will clear the tailing "/"
	// we need to add it back
	if p == "keys" {
		p = "keys/"
	}

	return p
}
