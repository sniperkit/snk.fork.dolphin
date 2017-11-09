package host

import (
	"net/http"

	"github.com/gorilla/mux"
	"we.com/dolphin/api/utils"
)

// Install deploy config handler
func Install(r *mux.Router) error {
	s := r.PathPrefix("/host").Subrouter()

	s.HandleFunc("/{env}/{type}/{name}", utils.HandlefuncWrap(add)).Methods(http.MethodPut)

	return nil
}

func add(w http.ResponseWriter, r *http.Request) (utils.Model, error) {

	return nil, nil
}

func delete(w http.ResponseWriter, r *http.Request) (utils.Model, error) {

	return nil, nil
}

func update(w http.ResponseWriter, r *http.Request) (utils.Model, error) {

	return nil, nil
}

func get(w http.ResponseWriter, r *http.Request) (utils.Model, error) {

	return nil, nil
}

func query(w http.ResponseWriter, r *http.Request) (utils.Model, error) {

	return nil, nil
}
