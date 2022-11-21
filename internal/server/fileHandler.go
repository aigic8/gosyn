package server

import (
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aigic8/gosyn/internal/server/utils"
	"github.com/cespare/xxhash"
	"github.com/gorilla/mux"
)

type fileHandler struct {
	Endpoints   map[string]string
	MaxHashSize int64
	logger      *utils.Logger
}

type FileGetHashResponse struct {
	Hash string `json:"hash"`
	File string `json:"file"`
}

func (fHandler *fileHandler) Get(w http.ResponseWriter, r *http.Request) {
	errh := utils.NewAPIErrHandler(fHandler.logger, r, w)
	vars := mux.Vars(r)
	fileVar := strings.TrimSpace(vars["file"])
	if fileVar == "" {
		errh.Warn(utils.ErrVarNotFound("file"))
		return
	}

	endpoint, filePath, err := splitEndpointAndFile(fileVar)
	if err != nil {
		errh.Warn(utils.ErrBadFileDesc(fileVar, err))
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(utils.ErrEndpointNotFound(endpoint))
		return
	}

	fullPath := path.Join(endpointPath, filePath)

	isSubPath, err := utils.IsSubPath(endpointPath, fullPath)
	if err != nil {
		errh.Err(utils.ErrUnknown("error checking subpath: " + err.Error()))
		return
	}
	if !isSubPath {
		errh.Err(utils.ErrOutOfEndpoint(fileVar, endpoint))
		return
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errh.Warn(utils.ErrFileNotFound(fileVar))
			return
		}
		errh.Warn(utils.ErrUnknown("err stating file: " + err.Error()))
		return
	}

	if stat.IsDir() {
		errh.Warn(utils.ErrPathIsDir(fileVar))
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		errh.Err(utils.ErrUnknown("err opening file: " + err.Error()))
		return
	}

	// TODO use brotli or gzip for text files
	io.Copy(w, file)
}

// TODO is it useful?
func (fHandler *fileHandler) GetHash(w http.ResponseWriter, r *http.Request) {
	// TODO get hash is very similar to GetFile should export similar functions
	errh := utils.NewAPIErrHandler(fHandler.logger, r, w)
	vars := mux.Vars(r)
	fileVar := strings.TrimSpace(vars["file"])

	if fileVar == "" {
		errh.Warn(utils.ErrVarNotFound("file"))
		return
	}

	endpoint, filePath, err := splitEndpointAndFile(fileVar)
	if err != nil {
		errh.Warn(utils.ErrBadFileDesc(fileVar, err))
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(utils.ErrEndpointNotFound(endpoint))
		return
	}

	fullPath := path.Join(endpointPath, filePath)

	isSubPath, err := utils.IsSubPath(endpointPath, fullPath)
	if err != nil {
		errh.Err(utils.ErrUnknown("error checking subpath: " + err.Error()))
		return
	}
	if !isSubPath {
		errh.Err(utils.ErrOutOfEndpoint(fileVar, endpoint))
		return
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errh.Warn(utils.ErrFileNotFound(fileVar))
			return
		}
		errh.Warn(utils.ErrUnknown("err stating file: " + err.Error()))
		return
	}

	if fileInfo.IsDir() {
		errh.Warn(utils.ErrPathIsDir(fileVar))
		return
	}

	if fHandler.MaxHashSize > 0 && fileInfo.Size() > fHandler.MaxHashSize {
		errh.Warn(utils.ErrFileTooBigToHash(fileVar, fHandler.MaxHashSize))
		return
	}

	hash, err := hashFile(fullPath)
	if err != nil {
		errh.Err(utils.ErrUnknown("err hashing file: " + err.Error()))
		return
	}

	respJson, err := wrapAPIResponse(FileGetHashResponse{Hash: hash, File: fileVar})
	if err != nil {
		errh.Err(utils.ErrUnknown("error marshaling json: " + err.Error()))
	}

	w.Write(respJson)
}

func (fHandler *fileHandler) AddNew(w http.ResponseWriter, r *http.Request) {
	errh := utils.NewAPIErrHandler(fHandler.logger, r, w)
	rawPath := strings.TrimSpace(r.Header.Get("x-file-path"))

	rawRecursive := strings.TrimSpace(r.Header.Get("x-recursive"))
	recursive := rawRecursive == "true"

	rawForce := strings.TrimSpace(r.Header.Get("x-force"))
	force := rawForce == "true"

	if rawPath == "" {
		errh.Warn(utils.ErrHeaderNotFound("filePath", "x-file-path"))
		return
	}

	endpoint, filePath, err := splitEndpointAndFile(rawPath)
	if err != nil {
		errh.Warn(utils.ErrBadFileDesc(rawPath, err))
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(utils.ErrEndpointNotFound(endpoint))
		return
	}

	fullPath := path.Join(endpointPath, filePath)

	isSubPath, err := utils.IsSubPath(endpointPath, fullPath)
	if err != nil {
		errh.Err(utils.ErrUnknown("error checking subpath: " + err.Error()))
		return
	}
	if !isSubPath {
		errh.Err(utils.ErrOutOfEndpoint(rawPath, endpoint))
		return
	}

	fileStat, err := os.Stat(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		dir := path.Dir(fullPath)
		if recursive {
			if err = os.MkdirAll(dir, 0777); err != nil {
				errh.Warn(utils.ErrUnknown("err making dir: " + err.Error()))
				return
			}
		}
		dirStat, err := os.Stat(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				errh.Warn(utils.ErrDirNotExist(dir))
				return
			}
			errh.Err(utils.ErrUnknown("err getting dir stat: " + err.Error()))
			return
		}
		if !dirStat.IsDir() {
			errh.Err(utils.ErrUnknown("err making dir: " + err.Error()))
			return
		}
	}

	if fileStat.IsDir() {
		errh.Warn(utils.ErrPathIsDir(rawPath))
		return
	}
	if !force {
		errh.Warn(utils.ErrFileExist(rawPath))
		return
	}

	// TODO maybe use smart transfer like rsync?
	file, err := os.Create(fullPath)
	if err != nil {
		errh.Err(utils.ErrUnknown("error creating file: " + err.Error()))
		return
	}

	defer r.Body.Close()
	if _, err = io.Copy(file, r.Body); err != nil {
		errh.Err(utils.ErrUnknown("error writing to file: " + err.Error()))
		return
	}

	respJson, err := wrapAPIResponse(map[string]string{})
	if err != nil {
		errh.Err(utils.ErrUnknown("error marshaling json: " + err.Error()))
		return
	}
	w.Write(respJson)
}

func splitEndpointAndFile(rawPath string) (string, string, error) {
	parts := strings.SplitN(rawPath, "/", 2)
	if len(parts) < 2 {
		return "", "", errors.New("endpoint is empty")
	}

	endpoint := strings.TrimSpace(parts[0])
	if endpoint == "" {
		return "", "", errors.New("endpoint is empty")
	}

	filePath := strings.TrimSpace(parts[1])
	if filePath == "" {
		return "", "", errors.New("file path is empty")
	}

	return endpoint, filePath, nil
}

func hashFile(filePath string) (string, error) {
	var hash string

	file, err := os.Open(filePath)
	if err != nil {
		return hash, err
	}
	defer file.Close()

	hashWriter := xxhash.New()
	if _, err := io.Copy(hashWriter, file); err != nil {
		return hash, err
	}
	hashBytes := hashWriter.Sum(nil)

	hash = hex.EncodeToString(hashBytes)
	return hash, nil
}
