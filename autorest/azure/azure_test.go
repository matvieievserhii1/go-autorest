package azure

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/mocks"
)

const (
	testLogPrefix = "azure:"
)

// Use a Client Inspector to set the request identifier.
func ExampleWithClientID() {
	uuid := "71FDB9F4-5E49-4C12-B266-DE7B4FD999A6"
	req, _ := autorest.Prepare(&http.Request{},
		autorest.AsGet(),
		autorest.WithBaseURL("https://microsoft.com/a/b/c/"))

	c := autorest.Client{Sender: mocks.NewSender()}
	c.RequestInspector = WithReturningClientID(uuid)

	c.Send(req)
	fmt.Printf("Inspector added the %s header with the value %s\n",
		HeaderClientID, req.Header.Get(HeaderClientID))
	fmt.Printf("Inspector added the %s header with the value %s\n",
		HeaderReturnClientID, req.Header.Get(HeaderReturnClientID))
	// Output:
	// Inspector added the x-ms-client-request-id header with the value 71FDB9F4-5E49-4C12-B266-DE7B4FD999A6
	// Inspector added the x-ms-return-client-request-id header with the value true
}

func TestWithReturningClientIDReturnsError(t *testing.T) {
	var errIn error
	uuid := "71FDB9F4-5E49-4C12-B266-DE7B4FD999A6"
	_, errOut := autorest.Prepare(&http.Request{},
		withErrorPrepareDecorator(&errIn),
		WithReturningClientID(uuid))

	if errOut == nil || errIn != errOut {
		t.Errorf("azure: WithReturningClientID failed to exit early when receiving an error -- expected (%v), received (%v)",
			errIn, errOut)
	}
}

func TestWithClientID(t *testing.T) {
	uuid := "71FDB9F4-5E49-4C12-B266-DE7B4FD999A6"
	req, _ := autorest.Prepare(&http.Request{},
		WithClientID(uuid))

	if req.Header.Get(HeaderClientID) != uuid {
		t.Errorf("azure: WithClientID failed to set %s -- expected %s, received %s",
			HeaderClientID, uuid, req.Header.Get(HeaderClientID))
	}
}

func TestWithReturnClientID(t *testing.T) {
	b := false
	req, _ := autorest.Prepare(&http.Request{},
		WithReturnClientID(b))

	if req.Header.Get(HeaderReturnClientID) != strconv.FormatBool(b) {
		t.Errorf("azure: WithReturnClientID failed to set %s -- expected %s, received %s",
			HeaderClientID, strconv.FormatBool(b), req.Header.Get(HeaderClientID))
	}
}

func TestExtractClientID(t *testing.T) {
	uuid := "71FDB9F4-5E49-4C12-B266-DE7B4FD999A6"
	resp := mocks.NewResponse()
	mocks.SetResponseHeader(resp, HeaderClientID, uuid)

	if ExtractClientID(resp) != uuid {
		t.Errorf("azure: ExtractClientID failed to extract the %s -- expected %s, received %s",
			HeaderClientID, uuid, ExtractClientID(resp))
	}
}

func TestExtractRequestID(t *testing.T) {
	uuid := "71FDB9F4-5E49-4C12-B266-DE7B4FD999A6"
	resp := mocks.NewResponse()
	mocks.SetResponseHeader(resp, HeaderRequestID, uuid)

	if ExtractRequestID(resp) != uuid {
		t.Errorf("azure: ExtractRequestID failed to extract the %s -- expected %s, received %s",
			HeaderRequestID, uuid, ExtractRequestID(resp))
	}
}

func TestIsAzureError_ReturnsTrueForAzureError(t *testing.T) {
	if !IsAzureError(&RequestError{}) {
		t.Errorf("azure: IsAzureError failed to return true for an Azure Service error")
	}
}

func TestIsAzureError_ReturnsFalseForNonAzureError(t *testing.T) {
	if IsAzureError(fmt.Errorf("An Error")) {
		t.Errorf("azure: IsAzureError return true for an non-Azure Service error")
	}
}

func TestNewErrorWithError_ReturnsUnwrappedError(t *testing.T) {
	e1 := RequestError{}
	e1.ServiceError = &ServiceError{Code: "42", Message: "A Message"}
	e1.StatusCode = 200
	e1.RequestID = "A RequestID"
	e2 := NewErrorWithError(&e1, "packageType", "method", nil, "message")

	if !reflect.DeepEqual(e1, e2) {
		t.Errorf("azure: NewErrorWithError wrapped an RequestError -- expected %T, received %T", e1, e2)
	}
}

func TestNewErrorWithError_WrapsAnError(t *testing.T) {
	e1 := fmt.Errorf("Inner Error")
	var e2 interface{} = NewErrorWithError(e1, "packageType", "method", nil, "message")

	if _, ok := e2.(RequestError); !ok {
		t.Errorf("azure: NewErrorWithError failed to wrap a standard error -- received %T", e2)
	}
}

func TestWithErrorUnlessStatusCode_NotAnAzureError(t *testing.T) {
	body := `<html>
		<head>
			<title>IIS Error page</title>
		</head>
		<body>Some non-JSON error page</body>
	</html>`
	r := mocks.NewResponseWithContent(body)
	r.Request = mocks.NewRequest()
	r.StatusCode = http.StatusBadRequest
	r.Status = http.StatusText(r.StatusCode)

	err := autorest.Respond(r,
		WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByClosing())
	ok, _ := err.(*RequestError)
	if ok != nil {
		t.Fatalf("azure: azure.RequestError returned from malformed response: %v", err)
	}

	// the error body should still be there
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != body {
		t.Fatalf("response body is wrong. got=%q exptected=%q", string(b), body)
	}
}

func TestWithErrorUnlessStatusCode_FoundAzureError(t *testing.T) {
	j := `{
		"error": {
			"code": "InternalError",
			"message": "Azure is having trouble right now."
		}
	}`
	uuid := "71FDB9F4-5E49-4C12-B266-DE7B4FD999A6"
	r := mocks.NewResponseWithContent(j)
	mocks.SetResponseHeader(r, HeaderRequestID, uuid)
	r.Request = mocks.NewRequest()
	r.StatusCode = http.StatusInternalServerError
	r.Status = http.StatusText(r.StatusCode)

	err := autorest.Respond(r,
		WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByClosing())

	if err == nil {
		t.Fatalf("azure: returned nil error for proper error response")
	}
	azErr, ok := err.(*RequestError)
	if !ok {
		t.Fatalf("azure: returned error is not azure.RequestError: %T", err)
	}
	if expected := "InternalError"; azErr.ServiceError.Code != expected {
		t.Fatalf("azure: wrong error code. expected=%q; got=%q", expected, azErr.ServiceError.Code)
	}
	if azErr.ServiceError.Message == "" {
		t.Fatalf("azure: error message is not unmarshaled properly")
	}
	if expected := http.StatusInternalServerError; azErr.StatusCode != expected {
		t.Fatalf("azure: got wrong StatusCode=%d Expected=%d", azErr.StatusCode, expected)
	}
	if expected := uuid; azErr.RequestID != expected {
		t.Fatalf("azure: wrong request ID in error. expected=%q; got=%q", expected, azErr.RequestID)
	}

	_ = azErr.Error()

	// the error body should still be there
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
	}
	if string(b) != j {
		t.Fatalf("response body is wrong. got=%q expected=%q", string(b), j)
	}

}

func withErrorPrepareDecorator(e *error) autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			*e = fmt.Errorf("autorest: Faux Prepare Error")
			return r, *e
		})
	}
}
