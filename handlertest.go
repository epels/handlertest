// Package handlertest lets you define test cases in concise code, or in YAML,
// and runs these against any HTTP handler.
package handlertest

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

type tt interface {
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Run(name string, f func(t *testing.T)) bool
}

// Assert the tt interface is implemented by *testing.T.
var _ tt = (*testing.T)(nil)

type TestCase struct {
	// Name can optionally be set to easily identify the test within the
	// Go test tool's output.
	Name     string
	Request  Request
	Response Response
}

// Request describes the request to fire at the HTTP handler.
type Request struct {
	Method string
	URL    string
	Body   string
}

// Response describes the expected response from the HTTP handler. All fields
// are optional: if they are not set, these are not asserted.
type Response struct {
	// Code is the expected HTTP status code.
	Code int
	// Body is the expected response body.
	Body string
}

// RunFromYAML reads a YAML serialized representation of TestCases from path
// and runs them against h. For locating path, the normal rules from os.Open
// are followed. If the file at path cannot be located or parsed, execution is
// stopped. If the response does not match the expectation, t is flagged as
// failed with a descriptive error.
func RunFromYAML(t tt, h http.Handler, path string) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("os: Open: %s", err)
		return
	}
	defer func() {
		_ = f.Close()
	}()

	runFromYAML(t, h, f)
}

func runFromYAML(t tt, h http.Handler, r io.Reader) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("io/ioutil: ReadAll: %s", err)
		return
	}

	var tcs []TestCase
	if err := yaml.Unmarshal(b, &tcs); err != nil {
		t.Fatalf("yaml: Unmarshal: %s", err)
		return
	}

	Run(t, h, tcs...)
}

// Run runs the test cases, tcs, against h. When the response does not match
// the expectation, t is flagged as failed with a descriptive error.
func Run(t tt, h http.Handler, tcs ...TestCase) {
	for _, tc := range tcs {
		f := func(t tt) {
			rec := httptest.NewRecorder()
			req := httpRequest(&tc.Request)
			h.ServeHTTP(rec, req)
			assertResponse(t, rec, &tc.Response)
		}

		if tc.Name != "" {
			t.Run(tc.Name, func(t *testing.T) {
				f(t)
			})
		} else {
			f(t)
		}
	}
}

func httpRequest(req *Request) *http.Request {
	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}
	return httptest.NewRequest(req.Method, req.URL, body)
}

func assertResponse(t tt, rec *httptest.ResponseRecorder, res *Response) {
	// If any of the fields in res have their types' zero value, we assume it's
	// not been set and should not be tested against.
	if res.Code != 0 && rec.Code != res.Code {
		t.Errorf("Got response code %d, expected %d", rec.Code, res.Code)
	}
	if s := rec.Body.String(); res.Body != "" && s != res.Body {
		t.Errorf("Got response body %q, expected %q", s, res.Body)
	}
}
