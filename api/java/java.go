/*
Sniperkit-Bot
- Status: analyzed
*/

package java

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Install java handler
func Install(r *mux.Router) error {
	s := r.PathPrefix("/java").Subrouter()

	s.HandleFunc("/deploy", nil)

	s.HandleFunc("/start", nil)

	s.HandleFunc("/stop", nil)

	s.HandleFunc("/probe/{type}/{name}", nil).Methods(http.MethodPost)
	s.HandleFunc("/probe/{type}/{name}", nil).Methods(http.MethodDelete)
	s.HandleFunc("/probe/{type}/{name}", nil).Methods(http.MethodGet)

	return nil
}

func projectInfo(w http.ResponseWriter, r *http.Request) error {

	return nil
}

func projectList(w http.ResponseWriter, r *http.Request) error {

	return nil
}

func stopInstance(w http.ResponseWriter, r *http.Request) error {

	return nil
}

func startInstance(w http.ResponseWriter, r *http.Request) error {

	return nil
}

func restartInstance(w http.ResponseWriter, r *http.Request) error {

	return nil
}

func setServiceRoute(w http.ResponseWriter, r *http.Request) error {

	return nil
}

func getServiceRoute(w http.ResponseWriter, r *http.Request) error {

	return nil
}
