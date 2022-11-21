package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aigic8/gosyn/internal/server/utils"
	"github.com/gorilla/mux"
)

type endpointHanlder struct {
	Endpoints map[string]string
	logger    *utils.Logger
}

// TODO maybe use pointers for LastMod and Size? since they can be empty
type TreePath struct {
	Name     string              `json:"name"`
	IsDir    bool                `json:"isDir"`
	Size     int64               `json:"size"`
	LastMod  time.Time           `json:"lastModifiction"`
	Children map[string]TreePath `json:"children"`
}

func (eHandler *endpointHanlder) Get(w http.ResponseWriter, r *http.Request) {
	errh := utils.NewAPIErrHandler(eHandler.logger, r, w)

	// TODO maybe a seperate validation layer?
	vars := mux.Vars(r)
	endpoint := strings.TrimSpace(vars["endpoint"])
	if endpoint == "" {
		errh.Warn(utils.ErrVarNotFound("endpoint"))
		return
	}

	endpointPath, endpointExists := eHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(utils.ErrEndpointNotFound("endpoint"))
		return
	}

	stat, err := os.Stat(endpointPath)
	if err != nil {
		errh.Warn(utils.ErrUnknown("error stating endpoint: " + err.Error()))
		return
	}

	if !stat.IsDir() {
		errh.Warn(utils.ErrUnknown(fmt.Sprintf("ednpoint '%s' is not a dir", endpointPath)))
		return
	}

	base, dirName := path.Split(endpointPath)

	tree := map[string]TreePath{
		dirName: {
			Name:     dirName,
			IsDir:    true,
			Size:     0,
			Children: map[string]TreePath{},
		},
	}

	if err := makeTree(base, tree); err != nil {
		errh.Err(utils.ErrUnknown("error making tree: " + err.Error()))
		return
	}

	jsonData, err := json.Marshal(tree)
	if err != nil {
		errh.Err(utils.ErrUnknown("error marshaling json: " + err.Error()))
		return
	}

	w.Write(jsonData)
}

func (eHandler *endpointHanlder) GetAll(w http.ResponseWriter, r *http.Request) {
	errh := utils.NewAPIErrHandler(eHandler.logger, r, w)

	endpoints := make([]string, 0, len(eHandler.Endpoints))
	for endpoint := range eHandler.Endpoints {
		endpoints = append(endpoints, endpoint)
	}
	jsonData, err := json.Marshal(endpoints)
	if err != nil {
		errh.Err(utils.ErrUnknown("error marshaling json: " + err.Error()))
		return
	}

	w.Write(jsonData)
}

func makeTree(base string, tree map[string]TreePath) error {
	for key, item := range tree {
		if !item.IsDir {
			continue
		}

		curr := path.Join(base, item.Name)
		children, err := os.ReadDir(curr)
		if err != nil {
			return err
		}

		for _, child := range children {
			var size int64
			var modeTime time.Time
			if !child.IsDir() {
				info, err := child.Info()
				if err != nil {
					return fmt.Errorf("error getting fileinfo: %v", err)
				}
				size = info.Size()
				modeTime = info.ModTime()
			}
			tree[key].Children[child.Name()] = TreePath{
				IsDir:    child.IsDir(),
				Name:     child.Name(),
				Size:     size,
				LastMod:  modeTime,
				Children: map[string]TreePath{},
			}
		}

		if err = makeTree(curr, tree[key].Children); err != nil {
			return err
		}
	}

	return nil
}
