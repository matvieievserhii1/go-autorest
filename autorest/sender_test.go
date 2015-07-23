package autorest

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/azure/go-autorest/autorest/mocks"
)

func ExampleSendWithSender() {
	client := mocks.NewClient()
	client.EmitStatus("202 Accepted", 202)

	logger := log.New(os.Stdout, "", 0)
	na := NullAuthorizer{}

	req, _ := Prepare(&http.Request{},
		AsGet(),
		WithBaseURL("https://microsoft.com/a/b/c/"),
		na.WithAuthorization())

	r, _ := SendWithSender(client, req,
		WithLogging("autorest", logger),
		DoErrorIfStatusCode(202),
		DoCloseIfError(),
		DoRetryForAttempts(5, time.Duration(0)))

	Respond(r,
		ByClosing())

	// Output:
	// autorest: Sending GET https://microsoft.com/a/b/c/
	// autorest: GET https://microsoft.com/a/b/c/ received 202 Accepted
	// autorest: Sending GET https://microsoft.com/a/b/c/
	// autorest: GET https://microsoft.com/a/b/c/ received 202 Accepted
	// autorest: Sending GET https://microsoft.com/a/b/c/
	// autorest: GET https://microsoft.com/a/b/c/ received 202 Accepted
	// autorest: Sending GET https://microsoft.com/a/b/c/
	// autorest: GET https://microsoft.com/a/b/c/ received 202 Accepted
	// autorest: Sending GET https://microsoft.com/a/b/c/
	// autorest: GET https://microsoft.com/a/b/c/ received 202 Accepted
}

func ExampleDoRetryForAttempts() {
	client := mocks.NewClient()
	client.EmitErrors(10)

	// Retry with backoff -- ensure returned Bodies are closed
	r, _ := SendWithSender(client, mocks.NewRequest(),
		DoCloseIfError(),
		DoRetryForAttempts(5, time.Duration(0)))

	Respond(r,
		ByClosing())

	fmt.Printf("Retry stopped after %d attempts", client.Attempts())
	// Output: Retry stopped after 5 attempts
}

func ExampleDoErrorIfStatusCode() {
	client := mocks.NewClient()
	client.EmitStatus("204 NoContent", 204)

	// Chain decorators to retry the request, up to five times, if the status code is 204
	r, _ := SendWithSender(client, mocks.NewRequest(),
		DoErrorIfStatusCode(204),
		DoCloseIfError(),
		DoRetryForAttempts(5, time.Duration(0)))

	Respond(r,
		ByClosing())

	fmt.Printf("Retry stopped after %d attempts with code %s", client.Attempts(), r.Status)
	// Output: Retry stopped after 5 attempts with code 204 NoContent
}

func TestSendWithSenderRunsDecoratorsInOrder(t *testing.T) {
	client := mocks.NewClient()
	s := ""

	r, err := SendWithSender(client, mocks.NewRequest(),
		withMessage(&s, "a"),
		withMessage(&s, "b"),
		withMessage(&s, "c"))
	if err != nil {
		t.Errorf("autorest: SendWithSender returned an error (%v)", err)
	}

	Respond(r,
		ByClosing())

	if s != "abc" {
		t.Errorf("autorest: SendWithSender invoke decorators out of order; expected 'abc', received '%s'", s)
	}
}

func TestAfterDelayWaits(t *testing.T) {
	client := mocks.NewClient()

	// Establish a baseline and then set the wait to 10x that amount
	// -- Waiting 10x the baseline should be long enough for a real test while not slowing the
	//    tests down too much
	tt := time.Now()
	SendWithSender(client, mocks.NewRequest())
	d := 10 * time.Since(tt)

	tt = time.Now()
	r, _ := SendWithSender(client, mocks.NewRequest(),
		AfterDelay(d))
	s := time.Since(tt)
	if s < d {
		t.Error("autorest: AfterDelay failed to wait for at least the specified duration")
	}

	Respond(r,
		ByClosing())
}

func TestAfterDelayDoesNotWaitTooLong(t *testing.T) {
	client := mocks.NewClient()

	// Establish a baseline and then set the wait to 10x that amount
	// -- Waiting 10x the baseline should be long enough for a real test while not slowing the
	//    tests down too much
	tt := time.Now()
	SendWithSender(client, mocks.NewRequest())
	d := 10 * time.Since(tt)

	tt = time.Now()
	r, _ := SendWithSender(client, mocks.NewRequest(),
		AfterDelay(d))
	s := time.Since(tt)
	if s > 5*d {
		t.Error("autorest: AfterDelay waited too long (more than five times the specified duration")
	}

	Respond(r,
		ByClosing())
}

func TestAsIs(t *testing.T) {
	client := mocks.NewClient()

	r1 := mocks.NewResponse()
	r2, err := SendWithSender(client, mocks.NewRequest(),
		AsIs(),
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					return r1, nil
				})
			}
		})())
	if err != nil {
		t.Errorf("autorest: AsIs returned an unexpected error (%v)", err)
	} else if !reflect.DeepEqual(r1, r2) {
		t.Errorf("autorest: AsIs modified the response -- received %v, expected %v", r2, r1)
	}

	Respond(r1,
		ByClosing())
	Respond(r2,
		ByClosing())
}

func TestDoCloseIfError(t *testing.T) {
	client := mocks.NewClient()
	client.EmitStatus("400 BadRequest", 400)

	r, _ := SendWithSender(client, mocks.NewRequest(),
		DoErrorIfStatusCode(400),
		DoCloseIfError())

	if r.Body.(*mocks.Body).IsOpen() {
		t.Error("autorest: Expected DoCloseIfError to close response body -- it was left open")
	}

	Respond(r,
		ByClosing())
}

func TestDoCloseIfErrorAcceptsNilResponse(t *testing.T) {
	client := mocks.NewClient()

	SendWithSender(client, mocks.NewRequest(),
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					resp, err := s.Do(r)
					if err != nil {
						resp.Body.Close()
					}
					return nil, fmt.Errorf("Faux Error")
				})
			}
		})(),
		DoCloseIfError())
}

func TestDoCloseIfErrorAcceptsNilBody(t *testing.T) {
	client := mocks.NewClient()

	SendWithSender(client, mocks.NewRequest(),
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					resp, err := s.Do(r)
					if err != nil {
						resp.Body.Close()
					}
					resp.Body = nil
					return resp, fmt.Errorf("Faux Error")
				})
			}
		})(),
		DoCloseIfError())
}

func TestDoErrorIfStatusCode(t *testing.T) {
	client := mocks.NewClient()
	client.EmitStatus("400 BadRequest", 400)

	r, err := SendWithSender(client, mocks.NewRequest(),
		DoErrorIfStatusCode(400),
		DoCloseIfError())
	if err == nil {
		t.Error("autorest: DoErrorIfStatusCode failed to emit an error for passed code")
	}

	Respond(r,
		ByClosing())
}

func TestDoErrorIfStatusCodeIgnoresStatusCodes(t *testing.T) {
	client := mocks.NewClient()
	client.EmitStatus("202 Accepted", 202)

	r, err := SendWithSender(client, mocks.NewRequest(),
		DoErrorIfStatusCode(400),
		DoCloseIfError())
	if err != nil {
		t.Error("autorest: DoErrorIfStatusCode failed to ignore a status code")
	}

	Respond(r,
		ByClosing())
}

func TestDoErrorUnlessStatusCode(t *testing.T) {
	client := mocks.NewClient()
	client.EmitStatus("400 BadRequest", 400)

	r, err := SendWithSender(client, mocks.NewRequest(),
		DoErrorUnlessStatusCode(202),
		DoCloseIfError())
	if err == nil {
		t.Error("autorest: DoErrorUnlessStatusCode failed to emit an error for an unknown status code")
	}

	Respond(r,
		ByClosing())
}

func TestDoErrorUnlessStatusCodeIgnoresStatusCodes(t *testing.T) {
	client := mocks.NewClient()
	client.EmitStatus("202 Accepted", 202)

	r, err := SendWithSender(client, mocks.NewRequest(),
		DoErrorUnlessStatusCode(202),
		DoCloseIfError())
	if err != nil {
		t.Error("autorest: DoErrorUnlessStatusCode emitted an error for a knonwn status code")
	}

	Respond(r,
		ByClosing())
}

func TestDoRetryForAttemptsStopsAfterAttempts(t *testing.T) {
	client := mocks.NewClient()
	client.EmitErrors(10)

	r, err := SendWithSender(client, mocks.NewRequest(),
		DoRetryForAttempts(5, time.Duration(0)),
		DoCloseIfError())
	if err == nil {
		t.Error("autorest: Mock client failed to emit errors")
	}

	Respond(r,
		ByClosing())

	if client.Attempts() != 5 {
		t.Error("autorest: DoRetryForAttempts failed to stop after specified number of attempts")
	}
}

func TestDoRetryForAttemptsReturnsResponse(t *testing.T) {
	client := mocks.NewClient()
	client.EmitErrors(1)

	r, err := SendWithSender(client, mocks.NewRequest(),
		DoRetryForAttempts(1, time.Duration(0)))
	if err == nil {
		t.Error("autorest: Mock client failed to emit errors")
	}

	if r == nil {
		t.Error("autorest: DoRetryForAttempts failed to return the underlying response")
	}

	Respond(r,
		ByClosing())
}

func TestDoRetryForDurationStopsAfterDuration(t *testing.T) {
	client := mocks.NewClient()
	client.EmitErrors(-1)

	d := 10 * time.Millisecond
	start := time.Now()
	r, err := SendWithSender(client, mocks.NewRequest(),
		DoRetryForDuration(d, time.Duration(0)),
		DoCloseIfError())
	if err == nil {
		t.Error("autorest: Mock client failed to emit errors")
	}

	Respond(r,
		ByClosing())

	if time.Now().Sub(start) < d {
		t.Error("autorest: DoRetryForDuration failed stopped too soon")
	}
}

func TestDoRetryForDurationStopsWithinReason(t *testing.T) {
	client := mocks.NewClient()
	client.EmitErrors(-1)

	d := 10 * time.Millisecond
	start := time.Now()
	r, err := SendWithSender(client, mocks.NewRequest(),
		DoRetryForDuration(d, time.Duration(0)),
		DoCloseIfError())
	if err == nil {
		t.Error("autorest: Mock client failed to emit errors")
	}

	Respond(r,
		ByClosing())

	if time.Now().Sub(start) > (5 * d) {
		t.Error("autorest: DoRetryForDuration failed stopped soon enough (exceeded 5 times specified duration)")
	}
}

func TestDoRetryForDurationReturnsResponse(t *testing.T) {
	client := mocks.NewClient()
	client.EmitErrors(-1)

	r, err := SendWithSender(client, mocks.NewRequest(),
		DoRetryForDuration(10*time.Millisecond, time.Duration(0)),
		DoCloseIfError())
	if err == nil {
		t.Error("autorest: Mock client failed to emit errors")
	}

	if r == nil {
		t.Error("autorest: DoRetryForDuration failed to return the underlying response")
	}

	Respond(r,
		ByClosing())
}

func TestDelayForBackoff(t *testing.T) {

	// Establish a baseline and then set the wait to 10x that amount
	// -- Waiting 10x the baseline should be long enough for a real test while not slowing the
	//    tests down too much
	tt := time.Now()
	DelayForBackoff(time.Millisecond, 0)
	d := 10 * time.Since(tt)

	start := time.Now()
	DelayForBackoff(d, 1)
	if time.Now().Sub(start) < d {
		t.Error("autorest: DelayForBackoff did not delay as long as expected")
	}
}

func TestDelayForBackoffWithinReason(t *testing.T) {

	// Establish a baseline and then set the wait to 10x that amount
	// -- Waiting 10x the baseline should be long enough for a real test while not slowing the
	//    tests down too much
	tt := time.Now()
	DelayForBackoff(time.Millisecond, 0)
	d := 10 * time.Since(tt)

	start := time.Now()
	DelayForBackoff(d, 1)
	if time.Now().Sub(start) > (time.Duration(5.0) * d) {
		t.Error("autorest: DelayForBackoff delayed too long (exceeded 5 times the specified duration)")
	}
}

func doEnsureBodyClosed(t *testing.T) SendDecorator {
	return func(s Sender) Sender {
		return SenderFunc(func(r *http.Request) (*http.Response, error) {
			resp, err := s.Do(r)
			if resp != nil && resp.Body != nil && resp.Body.(*mocks.Body).IsOpen() {
				t.Error("autorest: Expected Body to be closed -- it was left open")
			}
			return resp, err
		})
	}
}

func withMessage(output *string, msg string) SendDecorator {
	return func(s Sender) Sender {
		return SenderFunc(func(r *http.Request) (*http.Response, error) {
			resp, err := s.Do(r)
			if err == nil {
				*output += msg
			}
			return resp, err
		})
	}
}
