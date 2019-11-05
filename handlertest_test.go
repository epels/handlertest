package handlertest

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var emptyHandler = http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

type mock struct {
	errored bool
	fataled bool
	runFunc func(name string, f func(t *testing.T)) bool
}

func (m *mock) Errorf(format string, args ...interface{})  { m.errored = true }
func (m *mock) Fatalf(format string, args ...interface{})  { m.fataled = true }
func (m *mock) Run(name string, f func(t *testing.T)) bool { return m.runFunc(name, f) }

func TestRunFromYAML(t *testing.T) {
	t.Run("Fatal on non-existing file", func(t *testing.T) {
		var m mock
		RunFromYAML(&m, emptyHandler, "clearly/non/existing/file")

		if !m.fataled {
			t.Errorf("Got false, expected true")
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("With name: run called", func(t *testing.T) {
		var actual string
		m := mock{
			runFunc: func(name string, f func(t *testing.T)) bool {
				actual = name
				return true
			},
		}

		Run(&m, emptyHandler, TestCase{Name: "foo", Request: Request{URL: "http://localhost:8080"}})
		if actual != "foo" {
			t.Errorf("Got %q, expected foo", actual)
		}
	})

	t.Run("Without name: run not called", func(t *testing.T) {
		var actual string
		m := mock{
			runFunc: func(name string, f func(t *testing.T)) bool {
				actual = name
				return true
			},
		}

		Run(&m, emptyHandler, TestCase{Request: Request{URL: "http://localhost:8080"}})
		if actual != "" {
			t.Errorf("Got %q, expected empty string", actual)
		}
	})

	t.Run("Passing and failing test", func(t *testing.T) {
		var m mock
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("Bad")); err != nil {
				t.Logf("%T: Write: %s", w, err)
			}
		})

		pass := TestCase{
			Request: Request{
				Method: http.MethodGet,
				URL:    "/foo",
			},
			Response: Response{
				Code: http.StatusBadRequest,
				Body: "Bad",
			},
		}
		fail := TestCase{
			Request: Request{
				Method: http.MethodGet,
				URL:    "/foo",
			},
			Response: Response{
				Code: http.StatusInternalServerError, // Fail.
				Body: "Bad",
			},
		}
		Run(&m, h, pass, fail)

		if !m.errored {
			t.Errorf("Got false, expected true")
		}
	})

	t.Run("Single passing test", func(t *testing.T) {
		var m mock
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("Bad")); err != nil {
				t.Logf("%T: Write: %s", w, err)
			}
		})

		Run(&m, h, TestCase{
			Request: Request{
				Method: http.MethodGet,
				URL:    "/foo",
			},
			Response: Response{
				Code: http.StatusBadRequest,
				Body: "Bad",
			},
		})

		if m.errored {
			t.Errorf("Got true, expected false")
		}
	})

	t.Run("Single failing test", func(t *testing.T) {
		var m mock
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("Bad")); err != nil {
				t.Logf("%T: Write: %s", w, err)
			}
		})

		Run(&m, h, TestCase{
			Request: Request{
				Method: http.MethodGet,
				URL:    "/foo",
			},
			Response: Response{
				Code: http.StatusBadRequest,
				Body: "Also bad", // Fail.
			},
		})

		if !m.errored {
			t.Errorf("Got false, expected true")
		}
	})
}

func TestHTTPRequest(t *testing.T) {
	tt := []struct {
		name   string
		in     *Request
		expect *http.Request
	}{
		{
			name: "Simple GET",
			in: &Request{
				Method: http.MethodGet,
				URL:    "https://emilepels.nl",
			},
			expect: &http.Request{
				Method: http.MethodGet,
				URL:    mustParseURL(t, "https://emilepels.nl"),
			},
		},
		{
			name: "GET with query",
			in: &Request{
				Method: http.MethodGet,
				URL:    "https://emilepels.nl/foo?bar=baz&num=42",
			},
			expect: &http.Request{
				Method: http.MethodGet,
				URL:    mustParseURL(t, "https://emilepels.nl/foo?bar=baz&num=42"),
			},
		},
		{
			name: "POST without body",
			in: &Request{
				Method: http.MethodPost,
				URL:    "https://emilepels.nl/bar",
			},
			expect: &http.Request{
				Method: http.MethodPost,
				URL:    mustParseURL(t, "https://emilepels.nl/bar"),
			},
		},
		{
			name: "POST with body",
			in: &Request{
				Method: http.MethodPost,
				URL:    "https://emilepels.nl/bar",
				Body:   "Hello world!",
			},
			expect: &http.Request{
				Method: http.MethodPost,
				URL:    mustParseURL(t, "https://emilepels.nl/bar"),
				Body:   ioutil.NopCloser(strings.NewReader("Hello world!")),
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := httpRequest(tc.in)
			if got.Method != tc.expect.Method {
				t.Errorf("Got %q, expected %q", got.Method, tc.expect.Method)
			}
			// Comparing URLs based on String() is sufficient for our needs as
			// it includes scheme, host, path and query.
			if got.URL.String() != tc.expect.URL.String() {
				t.Errorf("Got %s, expected %s", got.URL, tc.expect.URL)
			}
			gotBody, expBody := readAll(t, got.Body), readAll(t, tc.expect.Body)
			if gotBody != expBody {
				t.Errorf("Got %q, expected %q", gotBody, expBody)
			}
		})
	}
}

func mustParseURL(t *testing.T, s string) *url.URL {
	t.Helper()

	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("net/url: Parse: %s", err)
	}
	return u
}

// readAll reads from rc, closes it, and then returns the contents. If an error
// occurs, t is failed.
func readAll(t *testing.T, rc io.ReadCloser) string {
	t.Helper()

	if rc == nil {
		return ""
	}
	defer func() {
		if err := rc.Close(); err != nil {
			t.Logf("%T: Close: %s", rc, err)
		}
	}()
	b, err := ioutil.ReadAll(rc)
	if err != nil {
		t.Fatalf("io/ioutil: ReadAll: %s", err)
	}
	return string(b)
}

func TestAssertResponse(t *testing.T) {
	tt := []struct {
		name string

		m     mock
		inRec *httptest.ResponseRecorder
		inRes *Response

		expectError bool
	}{
		{
			name: "OK with body",
			inRec: &httptest.ResponseRecorder{
				Code: http.StatusInternalServerError,
				Body: bytes.NewBufferString("Hello world!"),
			},
			inRes: &Response{
				Code: http.StatusInternalServerError,
				Body: "Hello world!",
			},
		},
		{
			name:  "OK without body",
			inRec: &httptest.ResponseRecorder{Code: http.StatusOK},
			inRes: &Response{Code: http.StatusOK},
		},
		{
			name: "Absent code and body",
			inRec: &httptest.ResponseRecorder{
				Code: http.StatusOK,
				Body: bytes.NewBufferString("Hello world!"),
			},
			inRes: &Response{},
		},
		{
			name: "Absent code",
			inRec: &httptest.ResponseRecorder{
				Code: http.StatusOK,
				Body: bytes.NewBufferString("Hello world!"),
			},
			inRes: &Response{
				Body: "Hello world!",
			},
		},
		{
			name: "Absent body",
			inRec: &httptest.ResponseRecorder{
				Code: http.StatusCreated,
				Body: bytes.NewBufferString("Hello world!"),
			},
			inRes: &Response{
				Code: http.StatusCreated,
			},
		},
		{
			name: "Code mismatch",
			inRec: &httptest.ResponseRecorder{
				Code: http.StatusInternalServerError,
				Body: bytes.NewBufferString("Hello world!"),
			},
			inRes: &Response{
				Code: http.StatusOK,
				Body: "Hello world!",
			},
			expectError: true,
		},
		{
			name: "Body mismatch",
			inRec: &httptest.ResponseRecorder{
				Code: http.StatusOK,
				Body: bytes.NewBufferString("Hello world!"),
			},
			inRes: &Response{
				Code: http.StatusOK,
				Body: "Not hello world",
			},
			expectError: true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assertResponse(&tc.m, tc.inRec, tc.inRes)
			if tc.m.errored != tc.expectError {
				t.Errorf("Got %t, expected %t", tc.m.errored, tc.expectError)
			}
		})
	}
}
