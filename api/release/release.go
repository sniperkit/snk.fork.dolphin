package release

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/go-multierror"

	log "github.com/golang/glog"
)

/*
all these api if related to release system( or boss)
and will only do file back,  transfer, and restore, will not
do all the other things like  clean cache, restart(start or stop) service
*/

var projectre = regexp.MustCompile("-release$")
var spaceCheck = regexp.MustCompile("[ \t]")

func publishFiles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars()
	project := vars["cluster"]
	project = projectre.ReplaceAllString(project, "")
	env := core.ENV(vars["env"])
	var err error
	urlquery := r.URL.Query()
	callback := urlquery.Get("callback")

	ab := checkPermisssion(r, "publish.file")
	if !ab {
		err = fmt.Errorf("not allowed")
		apiErr := apiError{
			typ: errorPermission,
			err: err,
		}

		respondError(w, apiErr, nil, callback)
		return
	}

	var merr *multierror.Error
	hoststr := urlquery.Get("hosts")
	if hoststr == "" {
		merr = multierror.Append(merr, fmt.Errorf("hosts is empty"))
	}
	hosts := strings.Split(hoststr, ",")
	location := urlquery.Get("location")
	repo := urlquery.Get("repo")
	from := urlquery.Get("from")
	if from == "" {
		merr = multierror.Append(merr, fmt.Errorf("from is empty"))
	}
	to := urlquery.Get("to")
	if to == "" {
		merr = multierror.Append(merr, fmt.Errorf("to is empty"))
	}

	rmdeletedfile := urlquery.Get("rmdeletedfile")
	var delfiles bool
	if rmdeletedfile == "1" {
		delfiles = true
	}

	if spaceCheck.MatchString(fmt.Sprintf("%s%s%s%s", repo, from, to, location)) {
		merr = multierror.Append(merr, fmt.Errorf("parameters cannot contain spaces"))
	}

	if err = merr.ErrorOrNil(); err != nil {
		apiErr := apiError{
			typ: errorBadData,
			err: err,
		}
		respondError(w, apiErr, nil, callback)
		return
	}

	// do the real work

	hostIDs := []core.UUID{}
	for _, v := range hosts {
		hostIDs = append(hostIDs, core.UUID(v))
	}

	gitRelease := file.GitRelease{
		Env:         env,
		Project:     project,
		Hosts:       hostIDs,
		From:        from,
		To:          to,
		Destination: location,
		DelFiles:    delfiles,
	}

	err = gitRelease.Validate()

	if err != nil {
		apiErr := apiError{
			typ: errorBadData,
			err: err,
		}
		respondError(w, apiErr, nil, callback)
		return
	}

	log.Infof("start to update project %s local repo", project)
	err = gitRelease.UpdateLocalRepo(1 * time.Minute)
	if err != nil {
		apiErr := apiError{
			typ: errorInternal,
			err: err,
		}
		respondError(w, apiErr, nil, callback)
		return
	}

	log.Infof("start to relase files from  project %s", project)
	out, err := gitRelease.Release()
	if err != nil {
		apiErr := apiError{
			typ: errorInternal,
			err: err,
		}
		respondError(w, apiErr, out, callback)
		return
	}

	respond(w, out, callback)
}

func createTicket(r *http.Reqeust) (interface{}, error}{
	vars := mux.Vars()


	return nil, nil
}
