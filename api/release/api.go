/*
Sniperkit-Bot
- Status: analyzed
*/

package release

import (
	"net/http"

	"github.com/gorilla/mux"
	"we.com/dolphin/types"
)

/*
	router.GET("/srvroute/:env/:project", getProjectRouteConfig)
	router.POST("/switchroute/:env/:project", setProjectRoute)
	router.GET("/stopService/:env/:project", stopService)
	router.GET("/startService/:env/:project/:program", startService)
	router.GET("/restartService/:env/:project/:program", restartService)
	router.GET("/dialService/:env/:project", dialNodes)
	router.GET("/getServiceInfo/:env/:project", getServiceInfo)
	router.GET("/getoutfile/:env/:project", getProjectOutfile)
	router.GET("/getoutfileContent/:env/:project", getProjectOutfileContent)
	router.GET("/listProjects/:ptype", getProjectListOfType)
	router.GET("/release/copyfile/:env/:project", publishFiles)
	router.GET("/reloadconfig", reloadConfig)
*/

// Install  relase handler
func Install(r *mux.Router) error {
	s := router.PathPrefix("/release").Subrouter()

	s.HandleFunc("/prepare/:type/:project/:ticket", utils.HandlefuncWrap(prepareTicket)).Methods(http.MethodPost)

	s.HandleFunc("/deploy", nil)

	s.HandleFunc("/start", nil)

	s.HandleFunc("/stop", nil)

	s.HandleFunc("/")

	s.HandleFunc("/release/copyfile/{env}/{cluster}", handerWrap(ah.saveProject)).Methods(http.MethodPost)
	return nil
}

func prepareTicket(r *http.Request) (interface{}, error)
	vars := mux.Vars(r)
	ptype := vars["type"]
	project := vars["project"]
	ticketNO := vars["ticket"]
	qs := r.URL.Query()
	branchs, ok := qs.Get("branches")
	if !ok {
		return nil, utils.BadData(errors.New(""))
	}

	dc, ok := getDeployConfig(types.ProjectType(ptype), project)
	if !ok {
		return "", nil
	}
}
