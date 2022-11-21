package server

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/aigic8/gosyn/internal/server/utils"
	"github.com/cespare/xxhash"
	"github.com/gorilla/mux"
	"gotest.tools/v3/assert"
)

type fileGetTestCase struct {
	Name     string
	File     string
	Status   int
	FileData []byte
}

func TestFileHandlerGet(t *testing.T) {
	base := t.TempDir()
	if err := mkDirs(base, []string{"normal/not-a-file"}); err != nil {
		panic(err)
	}

	normalFileData := []byte("I am totally normal")
	err := mkFiles(base, []fileInfo{
		{Path: "normal/file.txt", Data: normalFileData},
	})
	if err != nil {
		panic(err)
	}

	endpoints := map[string]string{
		"normal": path.Join(base, "normal"),
	}

	testCases := []fileGetTestCase{
		{Name: "normal", File: "normal/file.txt", Status: http.StatusOK, FileData: normalFileData},
		{Name: "file not exist", File: "normal/notexist.txt", Status: http.StatusNotFound},
		{Name: "endpoint not exist", File: "not-normal/endpoint.txt", Status: http.StatusNotFound},
		{Name: "file is dir", File: "normal/not-a-file", Status: http.StatusBadRequest},
		{Name: "empty file name", File: "  ", Status: http.StatusBadRequest},
	}

	logger, err := utils.NewLogger()
	if err != nil {
		panic(err)
	}
	fHandler := fileHandler{Endpoints: endpoints, MaxHashSize: 1000, logger: logger}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = mux.SetURLVars(r, map[string]string{"file": tc.File})
			w := httptest.NewRecorder()

			fHandler.Get(w, r)
			res := w.Result()

			defer res.Body.Close()
			assert.Equal(t, tc.Status, res.StatusCode)

			if tc.Status == http.StatusOK {
				resBody, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}

				assert.Equal(t, string(normalFileData), string(resBody))
			}
		})
	}
}

type fileHashTestCase struct {
	Name     string
	File     string
	Status   int
	FileHash string
}

func TestFileGetHash(t *testing.T) {
	base := t.TempDir()
	if err := mkDirs(base, []string{"normal/not-a-file"}); err != nil {
		panic(err)
	}

	normalFileData := []byte("I am totally normal")
	err := mkFiles(base, []fileInfo{
		{Path: "normal/file.txt", Data: normalFileData},
	})
	if err != nil {
		panic(err)
	}
	digest := xxhash.Sum64(normalFileData)
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, digest)
	normalFileHash := hex.EncodeToString(bytes)

	endpoints := map[string]string{
		"normal": path.Join(base, "normal"),
	}

	// TODO test max hash size

	testCases := []fileHashTestCase{
		{Name: "normal", File: "normal/file.txt", Status: http.StatusOK, FileHash: normalFileHash},
		{Name: "file not exist", File: "normal/notexist.txt", Status: http.StatusNotFound},
		{Name: "endpoint not exist", File: "not-normal/endpoint.txt", Status: http.StatusNotFound},
		{Name: "file is dir", File: "normal/not-a-file", Status: http.StatusBadRequest},
		{Name: "empty file name", File: "  ", Status: http.StatusBadRequest},
	}

	logger, err := utils.NewLogger()
	if err != nil {
		panic(err)
	}
	fHandler := fileHandler{Endpoints: endpoints, MaxHashSize: 50 * 1024 * 1024, logger: logger}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = mux.SetURLVars(r, map[string]string{"file": tc.File})
			w := httptest.NewRecorder()

			fHandler.GetHash(w, r)
			res := w.Result()

			defer res.Body.Close()
			assert.Equal(t, tc.Status, res.StatusCode)

			if tc.Status == http.StatusOK {
				resBody, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}

				assert.Equal(t, tc.FileHash, string(resBody))
			}
		})
	}
}
