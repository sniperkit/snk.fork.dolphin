/*
Sniperkit-Bot
- Status: analyzed
*/

package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/golang/glog"
)

type errorType string

const (
	errorNone       errorType = ""
	errorInternal             = "server_error"
	errorBadData              = "bad_data"
	errorPermission           = "not_allowed"
)

type apiError struct {
	typ errorType
	err error
}

// BadData create a new BadData error
func BadData(err error) error {
	if err == nil {
		return nil
	}
	return apiError{
		typ: errorBadData,
		err: err,
	}
}

//InternalErr create a new Internal error
func InternalErr(err error) error {
	if err == nil {
		return nil
	}
	return apiError{
		typ: errorInternal,
		err: err,
	}
}

// NotAllowed create a new NotAllowed error
func NotAllowed(err error) error {
	if err == nil {
		return nil
	}
	return apiError{
		typ: errorPermission,
		err: err,
	}
}

func (ae apiError) Error() string {
	return ae.err.Error()
}

type status string

const (
	statusSuccess status = "success"
	statusError          = "error"
)

type response struct {
	Status    status      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	ErrorType errorType   `json:"errorType,omitempty"`
	Error     string      `json:"error,omitempty"`
}

func respond(w http.ResponseWriter, data interface{}, callback string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)

	b, err := json.Marshal(&response{
		Status: statusSuccess,
		Data:   data,
	})
	if err != nil {
		return
	}
	if callback != "" {
		w.Write([]byte(callback + "("))
	}
	w.Write(b)
	if callback != "" {
		w.Write([]byte(")"))
	}
}

type Model interface{}

type handler func(w http.ResponseWriter, r *http.Request) (Model, error)

// HandlefuncWrap  wraps std http.HandleFunc
func HandlefuncWrap(hf handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := time.Now()
		callback := r.URL.Query().Get("callback")
		dat, err := hf(w, r)
		if err != nil {
			e, ok := err.(apiError)
			if !ok {
				e = apiError{
					typ: errorInternal,
					err: err,
				}
			}
			respondError(w, e, dat, callback)
		} else {
			respond(w, dat, callback)
		}
		e := time.Now()
		url := r.URL
		log.Infof("%v %v", url, e.Sub(s))
	}
}

func respondError(w http.ResponseWriter, apiErr apiError, data interface{}, callback string) {
	w.Header().Set("Content-Type", "application/json")

	switch apiErr.typ {
	case errorBadData:
		w.WriteHeader(http.StatusBadRequest)
	case errorInternal:
		w.WriteHeader(http.StatusInternalServerError)
	case errorPermission:
		w.WriteHeader(http.StatusUnauthorized)
	default:
		panic(fmt.Sprintf("unknown error type %q", apiErr))
	}

	b, err := json.Marshal(&response{
		Status:    statusError,
		ErrorType: apiErr.typ,
		Error:     apiErr.err.Error(),
		Data:      data,
	})
	if err != nil {
		log.Errorf("respondError: %+v", err)
		return
	}
	log.Errorf("api error: %+v, %v", apiErr, apiErr.Error())

	if callback != "" {
		w.Write([]byte(callback + "("))
	}
	w.Write(b)
	if callback != "" {
		w.Write([]byte(")"))
	}
}

// Receive unmarsher json from request body
func Receive(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	err := dec.Decode(v)
	if err != nil {
		log.V(10).Infof("Decoding request failed: %v", err)
	}
	return err
}

func checkPermisssion(r *http.Request, permission string) bool {
	return true
}
