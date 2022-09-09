package httpx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gabriel-vasile/mimetype"
	"github.com/nyaruka/gocommon/dates"
	"github.com/pkg/errors"
)

// Do makes the given HTTP request using the current requestor and retry config
func Do(client *http.Client, request *http.Request, retries *RetryConfig, access *AccessConfig) (*http.Response, error) {
	r, _, err := do(client, request, retries, access)
	return r, err
}

func do(client *http.Client, request *http.Request, retries *RetryConfig, access *AccessConfig) (*http.Response, int, error) {
	if access != nil {
		allowed, err := access.Allow(request)
		if err != nil {
			return nil, 0, err
		}
		if !allowed {
			return nil, 0, errors.Errorf("request to %s denied", request.URL.Hostname())
		}
	}

	var response *http.Response
	var err error
	retry := 0

	for {
		response, err = currentRequestor.Do(client, request)

		if retries != nil && retry < retries.MaxRetries() {
			backoff := retries.Backoff(retry)

			if retries.ShouldRetry(request, response, backoff) {
				time.Sleep(backoff)
				retry++
				continue
			}
		}

		break
	}

	return response, retry, err
}

// Trace holds the complete trace of an HTTP request/response
type Trace struct {
	Request       *http.Request
	RequestTrace  []byte
	Response      *http.Response
	ResponseTrace []byte
	ResponseBody  []byte // response body stored separately
	StartTime     time.Time
	EndTime       time.Time
	Retries       int
}

func (t *Trace) String() string {
	b := &strings.Builder{}
	b.WriteString(fmt.Sprintf(">>>>>>>> %s %s\n", t.Request.Method, t.Request.URL))
	b.WriteString(string(t.RequestTrace))
	b.WriteString("\n<<<<<<<<\n")
	b.WriteString(string(t.ResponseTrace))
	b.WriteString(string(t.ResponseBody))
	return b.String()
}

// SanitizedRequest returns a valid UTF-8 string version of the request, substituting the body with a placeholder
// if it isn't valid UTF-8. It also strips any NULL characters as not all external dependencies can handle those.
func (t *Trace) SanitizedRequest(placeholder string) string {
	// split request trace into headers and body
	var headers, body []byte
	parts := bytes.SplitN(t.RequestTrace, []byte("\r\n\r\n"), 2)
	headers = append(parts[0], []byte("\r\n\r\n")...)

	if len(parts) > 1 {
		body = parts[1]
	} else {
		body = nil
	}

	return santizedTrace(headers, body, placeholder)
}

// SanitizedResponse returns a valid UTF-8 string version of the response, substituting the body with a placeholder
// if it isn't valid UTF-8. It also strips any NULL characters as not all external dependencies can handle those.
func (t *Trace) SanitizedResponse(placeholder string) string {
	return santizedTrace(t.ResponseTrace, t.ResponseBody, placeholder)
}

func santizedTrace(header []byte, body []byte, bodyPlaceHolder string) string {
	b := &bytes.Buffer{}

	// ensure headers section is valid
	b.Write(replaceNullChars(bytes.ToValidUTF8(header, []byte(`�`))))

	// only include body if it's valid UTF-8 as it could be a binary file or anything
	if utf8.Valid(body) {
		b.Write(replaceNullChars(body))
	} else {
		b.Write([]byte(bodyPlaceHolder))
	}

	return b.String()
}

func replaceNullChars(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte{0}, []byte(`�`))
}

// DoTrace makes the given request saving traces of the complete request and response
func DoTrace(client *http.Client, request *http.Request, retries *RetryConfig, access *AccessConfig, maxBodyBytes int) (*Trace, error) {
	requestTrace, err := httputil.DumpRequestOut(request, true)
	if err != nil {
		return nil, err
	}

	trace := &Trace{
		Request:      request,
		RequestTrace: requestTrace,
		StartTime:    dates.Now(),
	}

	response, retryCount, err := do(client, request, retries, access)
	trace.EndTime = dates.Now()
	trace.Retries = retryCount

	if err != nil {
		return trace, err
	}

	trace.Response = response
	trace.ResponseTrace, trace.ResponseBody, err = dumpResponse(response, maxBodyBytes)
	if err != nil {
		return trace, err
	}

	return trace, nil
}

// NewRequest is a convenience method to create a request with the given headers
func NewRequest(method string, url string, body io.Reader, headers map[string]string) (*http.Request, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		r.Header.Set(key, value)
	}

	return r, nil
}

func dumpResponse(response *http.Response, maxBodyBytes int) ([]byte, []byte, error) {
	// dump response trace without body which will be parsed separately
	responseTrace, err := httputil.DumpResponse(response, false)
	if err != nil {
		return nil, nil, err
	}
	responseBody, err := readBody(response, maxBodyBytes)
	if err != nil {
		return responseTrace, nil, err
	}

	return responseTrace, responseBody, nil
}

// attempts to read the body of an HTTP response
func readBody(response *http.Response, maxBodyBytes int) ([]byte, error) {
	defer response.Body.Close()

	if maxBodyBytes > 0 {
		// we will only read up to our max body bytes limit
		bodyReader := io.LimitReader(response.Body, int64(maxBodyBytes)+1)

		bodyBytes, err := io.ReadAll(bodyReader)
		if err != nil {
			return nil, err
		}

		// if we have no remaining bytes, error because the body was too big
		if bodyReader.(*io.LimitedReader).N <= 0 {
			return nil, errors.Errorf("webhook response body exceeds %d bytes limit", maxBodyBytes)
		}

		return bodyBytes, nil
	}

	// if there is no limit, read the entire body
	return io.ReadAll(response.Body)
}

// Requestor is anything that can make an HTTP request with a client
type Requestor interface {
	Do(*http.Client, *http.Request) (*http.Response, error)
}

type defaultRequestor struct{}

func (r defaultRequestor) Do(client *http.Client, request *http.Request) (*http.Response, error) {
	return client.Do(request)
}

// DefaultRequestor is the default HTTP requestor
var DefaultRequestor Requestor = defaultRequestor{}
var currentRequestor = DefaultRequestor

// SetRequestor sets the requestor used by Request
func SetRequestor(requestor Requestor) {
	currentRequestor = requestor
}

// DetectContentType is a drop in replacement for http.DetectContentType which leans on
// the github.com/gabriel-vasile/mimetype library
func DetectContentType(d []byte) string {
	return mimetype.Detect(d).String()
}

// BasicAuth returns the Authorization header value for HTTP Basic auth
func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
