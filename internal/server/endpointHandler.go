package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type endpointHanlder struct {
	Endpoints map[string]string
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
	// TODO maybe a seperate validation layer?
	vars := mux.Vars(r)
	endpoint := strings.TrimSpace(vars["endpoint"])
	if endpoint == "" {
		w.WriteHeader(http.StatusBadRequest)
		// TODO better error handling and logging
		w.Write([]byte("endpoint is empty"))
		log.Print("endpoint is empty")
		return
	}

	endpointPath, endpointExists := eHandler.Endpoints[endpoint]
	if !endpointExists {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("endpoint does not exist"))
		log.Println("endpoint does not exist")
		return
	}

	stat, err := os.Stat(endpointPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("there is a problem with this endpoint"))
		log.Printf("error stating endpoint path '%s': %v\n", endpointPath, err)
		return
	}

	if !stat.IsDir() {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("there is a problem with this endpoint"))
		log.Printf("endpoint '%s' is not a dir\n", endpointPath)
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
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error happened"))
		log.Printf("error making tree: %v", err)
		return
	}

	jsonData, err := json.Marshal(tree)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error happened"))
		log.Printf("error marshaling json: %v", err)
		return
	}

	w.Write(jsonData)
}

func (eHandler *endpointHanlder) GetAll(w http.ResponseWriter, r *http.Request) {
	endpoints := make([]string, 0, len(eHandler.Endpoints))
	for endpoint := range eHandler.Endpoints {
		endpoints = append(endpoints, endpoint)
	}
	jsonData, err := json.Marshal(endpoints)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error happened"))
		log.Printf("error marshaling json: %v", err)
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
