# handlertest [![GoDoc](https://godoc.org/github.com/epels/handlertest?status.svg)](http://godoc.org/github.com/epels/handlertest)

Go has great test tooling. Although`httptest` makes testing your HTTP handlers convenient, it can still result in verbose test code.
This package, `handlertest`, seamlessly integrates with the Go tools you already know, but removes that boilerplate.

Tests define what request to send, and what response is expected. This can be done in either (Go) code, or in YAML. It runs within the process itself and runs against any `http.Handler`, which gives you the flexibility to wire up your service as you would in any other tests.

## Example

Let's say we're testing a handler that exposes a simple health check endpoint, which looks like this:

```go
func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("ok"))
}
```

To test this, we would normally have to manually construct a `httptest.ResponseRecorder` and `*http.Request`. We would then manually pass this to our handler's `ServeHTTP` method, and then assert the relevant aspects of the response. This is a lot easier to do with `handlertest` though. We'll now look at the two approaches - code, and YAML.

### Code

```go
func TestHealth(t *testing.T) {
	h := handler()
	handlertest.Run(t, h, handlertest.TestCase{
		Name: "Health returns OK",
		Request: handlertest.Request{
			Method: http.MethodGet,
			URL:    "/health",
		},
		Response: handlertest.Response{
			Code: http.StatusOK,
			Body: "ok",
		},
	})
}
```

`Run()` is variadic, so any number of test cases can be passed.

### YAML

It's even easier to do this with some straightforward YAML. Here's an example with a couple more test cases:

```yaml
# testdata/health.yaml
- name: "Health returns OK"
  request:
    method: "GET"
    url: "/health"
  response:
    body: "ok"
- name: "Health returns bad method on POST"
  request:
    method: "POST"
    url: "/health"
  response:
    code: 405
- name: "Router returns not found on undefined URL"
  request:
    method: "GET"
    url: "/health/foo"
  response:
    code: 404
```

From our test code, we'll only need to point `handlertest` towards this YAML file:

```go
func TestHealth(t *testing.T) {
	h := handler()
	handlertest.RunFromYAML(t, h, "testdata/health.yaml")
}
```

To make this as painless as possible, you won't even have to deal with opening and parsing the file. If something unexpected happens, e.g. the YAML cannot be parsed, the test will be marked as failed with a descriptive error message.

Running the test cases defined in this YAML file against the handler we created above yields the following result:

```
pels$ go test -v ./...
=== RUN   TestHealth
=== RUN   TestHealth/Health_returns_OK
=== RUN   TestHealth/Health_returns_bad_method_on_POST
=== RUN   TestHealth/Router_returns_not_found_on_undefined_URL
--- FAIL: TestHealth (0.00s)
    --- PASS: TestHealth/Health_returns_OK (0.00s)
    --- FAIL: TestHealth/Health_returns_bad_method_on_POST (0.00s)
        handlertest.go:92: Got response code 200, expected 405
    --- PASS: TestHealth/Router_returns_not_found_on_undefined_URL (0.00s)
FAIL
FAIL	github.com/epels/example	0.022s
FAIL
``` 

As you can see, this package plays nicely with the Go test tool. 

## Credits

This project depends on the excellent [`go-yaml/yaml`](https://github.com/go-yaml/yaml) package.
