package server

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aigic8/gosyn/internal/server/log"
	"github.com/aigic8/gosyn/internal/server/utils"
	"github.com/gorilla/mux"
)

type endpointHanlder struct {
	Endpoints map[string]string
	logger    *log.Logger
}

type (
	EndpointGetAllResponse struct {
		Endpoints []string `json:"endpoints"`
	}

	EndpointGetResponse struct {
		Tree map[string]utils.TreePath `json:"tree"`
	}
)

func (eHandler *endpointHanlder) Get(w http.ResponseWriter, r *http.Request) {
	errh := log.NewAPIErrHandler(eHandler.logger, r, w)

	// TODO maybe a seperate validation layer?
	vars := mux.Vars(r)
	endpoint := strings.TrimSpace(vars["endpoint"])
	if endpoint == "" {
		errh.Warn(log.ErrVarNotFound("endpoint"))
		return
	}

	endpointPath, endpointExists := eHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(log.ErrEndpointNotFound(endpoint))
		return
	}

	stat, err := os.Stat(endpointPath)
	if err != nil {
		errh.Warn(log.ErrUnknown("error stating endpoint: " + err.Error()))
		return
	}

	if !stat.IsDir() {
		errh.Warn(log.ErrUnknown(fmt.Sprintf("ednpoint '%s' is not a dir", endpointPath)))
		return
	}

	base, dirName := path.Split(endpointPath)

	tree := map[string]utils.TreePath{
		dirName: {
			Name:     dirName,
			IsDir:    true,
			Size:     0,
			Children: map[string]utils.TreePath{},
		},
	}

	// TODO can be too expensive is the tree is too big? maybe add something like depth to it?
	if err := utils.MakeTree(base, tree); err != nil {
		errh.Err(log.ErrUnknown("error making tree: " + err.Error()))
		return
	}

	jsonData, err := wrapAPIResponse(EndpointGetResponse{Tree: tree})
	if err != nil {
		errh.Err(log.ErrUnknown("error marshaling json: " + err.Error()))
		return
	}

	w.Write(jsonData)
}

func (eHandler *endpointHanlder) GetAll(w http.ResponseWriter, r *http.Request) {
	errh := log.NewAPIErrHandler(eHandler.logger, r, w)

	endpoints := make([]string, 0, len(eHandler.Endpoints))
	for endpoint := range eHandler.Endpoints {
		endpoints = append(endpoints, endpoint)
	}
	jsonData, err := wrapAPIResponse(EndpointGetAllResponse{Endpoints: endpoints})
	if err != nil {
		errh.Err(log.ErrUnknown("error marshaling json: " + err.Error()))
		return
	}

	w.Write(jsonData)
}
