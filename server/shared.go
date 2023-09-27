package server

import (
	"encoding/json"
	"io"
	"net/http"

	client "wamp3go"
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
		payload = client.ErrorPayload{e.Error()}
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

// func verifyToken(v string) error {
// 	queryMap := r.URL.Query()
// 	tokenList := queryMap["token"]
// 	if len(tokenList) > 0 {
// 		token := tokenList[0]
// 		if len(token) > 0 {
// 			return nil
// 		} else {
// 			return errors.New("AuthenticationError")
// 		}
// 	} else {
// 		return errors.New("TokenRequired")
// 	}
// }
