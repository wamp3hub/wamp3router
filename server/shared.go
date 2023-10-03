package server

import (
	"encoding/json"
	"io"
	"net/http"

	interview "github.com/wamp3hub/wamp3go/transport/interview"
)

func readJSONBody(requestBody io.ReadCloser, v any) error {
	requestBodyBytes, e := io.ReadAll(requestBody)
	if e == nil {
		requestBody.Close()
		e = json.Unmarshal(requestBodyBytes, v)
	}
	return e
}

func writeJSONBody(
	w http.ResponseWriter,
	statusCode int,
	payload any,
) error {
	e, isError := payload.(error)
	if isError {
		payload = interview.ErrorPayload{e.Error()}
	}
	responseBodyBytes, e := json.Marshal(payload)
	if e == nil {
		responseHeaders := w.Header()
		responseHeaders.Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(responseBodyBytes)
	}
	return e
}

func jsonEndpoint(
	procedure func(*http.Request) (int, any),
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode, payload := procedure(r)
		writeJSONBody(w, statusCode, payload)
	}
}
