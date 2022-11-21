package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/aigic8/gosyn/internal/server/utils"
	"github.com/gorilla/mux"
	"gotest.tools/v3/assert"
)

type endpointGetTestCase struct {
	Name     string
	Endpoint string
	Status   int
	Tree     map[string]TreePath
}

type fileInfo struct {
	Path string
	Data []byte
}

func TestEndpointGet(t *testing.T) {
	base := t.TempDir()

	err := mkDirs(base, []string{"seether", "pink-floyd/freq"})
	if err != nil {
		panic(err)
	}

	err = mkFiles(base, []fileInfo{
		{
			Path: "seether/truth.txt",
			Data: []byte("No, there's nothing you say that can salvage the lie, but I'm trying to keep my intentions disguised"),
		},
		{
			Path: "pink-floyd/freq/time.txt",
			Data: []byte("The time is gone, the song is over, thought I'd something more to say"),
		},
		{
			Path: "pink-floyd/wish-you-where-here.txt",
			Data: []byte("Swimming in a fish bowl. Year after year. Running over the same old ground. What have we found? The same old fears."),
		},
		{Path: "random", Data: []byte("")},
	})
	if err != nil {
		panic(err)
	}

	infos, err := getInfos(base, []string{"seether/truth.txt", "pink-floyd/freq/time.txt", "pink-floyd/wish-you-where-here.txt"})
	if err != nil {
		panic(err)
	}

	truthInfo := infos["seether/truth.txt"]
	timeInfo := infos["pink-floyd/freq/time.txt"]
	wereHereInfo := infos["pink-floyd/wish-you-where-here.txt"]

	endpoints := map[string]string{
		"seether": path.Join(base, "seether"),
		"pink":    path.Join(base, "pink-floyd"),
		"random":  path.Join(base, "random"),
	}

	normalTree := map[string]TreePath{
		"seether": {Name: "seether", IsDir: true, Children: map[string]TreePath{
			"truth.txt": {
				Name:     "truth.txt",
				IsDir:    false,
				Size:     truthInfo.Size(),
				LastMod:  truthInfo.ModTime(),
				Children: map[string]TreePath{},
			},
		}},
	}
	recursiveTree := map[string]TreePath{
		"pink-floyd": {Name: "pink-floyd", IsDir: true, Children: map[string]TreePath{
			"freq": {Name: "freq", IsDir: true, Children: map[string]TreePath{
				"time.txt": {
					Name:     "time.txt",
					IsDir:    false,
					Size:     timeInfo.Size(),
					LastMod:  timeInfo.ModTime(),
					Children: map[string]TreePath{}},
			}},
			"wish-you-where-here.txt": {
				Name:     "wish-you-where-here.txt",
				IsDir:    false,
				Size:     wereHereInfo.Size(),
				LastMod:  wereHereInfo.ModTime(),
				Children: map[string]TreePath{},
			},
		}},
	}

	cases := []endpointGetTestCase{
		{Name: "normal", Endpoint: "seether", Status: http.StatusOK, Tree: normalTree},
		{Name: "recursive endpoint", Endpoint: "pink", Status: http.StatusOK, Tree: recursiveTree},
		{Name: "not exist", Endpoint: "lalaland", Status: http.StatusNotFound},
		{Name: "file endpoint", Endpoint: "random", Status: http.StatusInternalServerError},
	}

	logger, err := utils.NewLogger()
	if err != nil {
		panic(err)
	}
	eHandler := endpointHanlder{Endpoints: endpoints, logger: logger}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			r = mux.SetURLVars(r, map[string]string{"endpoint": tc.Endpoint})
			eHandler.Get(w, r)

			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, tc.Status, res.StatusCode)

			if tc.Status == http.StatusOK {
				data, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}

				respTree := map[string]TreePath{}
				if err = json.Unmarshal(data, &respTree); err != nil {
					panic(err)
				}
				assert.DeepEqual(t, tc.Tree, respTree)
			}
		})
	}
}

func TestEndpointGetAll(t *testing.T) {
	endpoints := map[string]string{
		"seether": "seether",
		"pink":    "pink-floyd",
		"kaboos":  "songs/kaboos",
	}

	logger, err := utils.NewLogger()
	if err != nil {
		panic(err)
	}
	eHandler := endpointHanlder{Endpoints: endpoints, logger: logger}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	eHandler.GetAll(w, r)

	res := w.Result()
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	data, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	expectedEndpoints := []string{}
	for endpoint := range endpoints {
		expectedEndpoints = append(expectedEndpoints, endpoint)
	}
	resEndpoints := []string{}
	json.Unmarshal(data, &resEndpoints)
	assert.Equal(t, true, arrsAreEqual(expectedEndpoints, resEndpoints))
}

func mkDirs(base string, dirs []string) error {
	for _, dir := range dirs {
		err := os.MkdirAll(path.Join(base, dir), 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func mkFiles(base string, files []fileInfo) error {
	for _, file := range files {
		err := os.WriteFile(path.Join(base, file.Path), file.Data, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func getInfos(base string, filePaths []string) (map[string]os.FileInfo, error) {
	res := map[string]os.FileInfo{}
	for _, filePath := range filePaths {
		info, err := os.Stat(path.Join(base, filePath))
		if err != nil {
			return nil, err
		}
		res[filePath] = info
	}
	return res, nil
}

func arrsAreEqual[T comparable](arr1 []T, arr2 []T) bool {
	if len(arr1) != len(arr2) {
		return false
	}

	arr2map := map[T]bool{}
	for _, item := range arr2 {
		arr2map[item] = true
	}

	for _, item := range arr1 {
		if _, ok := arr2map[item]; !ok {
			return false
		}
	}
	return true
}
