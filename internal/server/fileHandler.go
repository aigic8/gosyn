package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aigic8/gosyn/internal/server/log"
	"github.com/aigic8/gosyn/internal/server/utils"
	"github.com/gorilla/mux"
)

type fileHandler struct {
	Endpoints   map[string]string
	MaxHashSize int64
	logger      *log.Logger
}

type FileGetHashResponse struct {
	Hash string `json:"hash"`
	File string `json:"file"`
}

func (fHandler *fileHandler) Get(w http.ResponseWriter, r *http.Request) {
	errh := log.NewAPIErrHandler(fHandler.logger, r, w)
	vars := mux.Vars(r)
	fileVar := strings.TrimSpace(vars["file"])
	if fileVar == "" {
		errh.Warn(log.ErrVarNotFound("file"))
		return
	}

	endpoint, filePath, err := utils.SplitEndpointAndFile(fileVar)
	if err != nil {
		errh.Warn(log.ErrBadFileDesc(fileVar, err))
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(log.ErrEndpointNotFound(endpoint))
		return
	}

	fullPath := path.Join(endpointPath, filePath)

	isSubPath, err := utils.IsSubPath(endpointPath, fullPath)
	if err != nil {
		errh.Err(log.ErrUnknown("error checking subpath: " + err.Error()))
		return
	}
	if !isSubPath {
		errh.Err(log.ErrOutOfEndpoint(fileVar, endpoint))
		return
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errh.Warn(log.ErrFileNotFound(fileVar))
			return
		}
		errh.Warn(log.ErrUnknown("err stating file: " + err.Error()))
		return
	}

	if stat.IsDir() {
		errh.Warn(log.ErrPathIsDir(fileVar))
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		errh.Err(log.ErrUnknown("err opening file: " + err.Error()))
		return
	}

	// TODO use brotli or gzip for text files
	io.Copy(w, file)
}

// TODO is it useful?
func (fHandler *fileHandler) GetHash(w http.ResponseWriter, r *http.Request) {
	// TODO get hash is very similar to GetFile should export similar functions
	errh := log.NewAPIErrHandler(fHandler.logger, r, w)
	vars := mux.Vars(r)
	fileVar := strings.TrimSpace(vars["file"])

	if fileVar == "" {
		errh.Warn(log.ErrVarNotFound("file"))
		return
	}

	endpoint, filePath, err := utils.SplitEndpointAndFile(fileVar)
	if err != nil {
		errh.Warn(log.ErrBadFileDesc(fileVar, err))
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(log.ErrEndpointNotFound(endpoint))
		return
	}

	fullPath := path.Join(endpointPath, filePath)

	isSubPath, err := utils.IsSubPath(endpointPath, fullPath)
	if err != nil {
		errh.Err(log.ErrUnknown("error checking subpath: " + err.Error()))
		return
	}
	if !isSubPath {
		errh.Err(log.ErrOutOfEndpoint(fileVar, endpoint))
		return
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errh.Warn(log.ErrFileNotFound(fileVar))
			return
		}
		errh.Warn(log.ErrUnknown("err stating file: " + err.Error()))
		return
	}

	if fileInfo.IsDir() {
		errh.Warn(log.ErrPathIsDir(fileVar))
		return
	}

	if fHandler.MaxHashSize > 0 && fileInfo.Size() > fHandler.MaxHashSize {
		errh.Warn(log.ErrFileTooBigToHash(fileVar, fHandler.MaxHashSize))
		return
	}

	hash, err := utils.HashFile(fullPath)
	if err != nil {
		errh.Err(log.ErrUnknown("err hashing file: " + err.Error()))
		return
	}

	respJson, err := wrapAPIResponse(FileGetHashResponse{Hash: hash, File: fileVar})
	if err != nil {
		errh.Err(log.ErrUnknown("error marshaling json: " + err.Error()))
	}

	w.Write(respJson)
}

func (fHandler *fileHandler) AddNew(w http.ResponseWriter, r *http.Request) {
	errh := log.NewAPIErrHandler(fHandler.logger, r, w)
	rawPath := strings.TrimSpace(r.Header.Get("x-file-path"))

	rawRecursive := strings.TrimSpace(r.Header.Get("x-recursive"))
	recursive := rawRecursive == "true"

	rawForce := strings.TrimSpace(r.Header.Get("x-force"))
	force := rawForce == "true"

	if rawPath == "" {
		errh.Warn(log.ErrHeaderNotFound("filePath", "x-file-path"))
		return
	}

	endpoint, filePath, err := utils.SplitEndpointAndFile(rawPath)
	if err != nil {
		errh.Warn(log.ErrBadFileDesc(rawPath, err))
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		errh.Warn(log.ErrEndpointNotFound(endpoint))
		return
	}

	fullPath := path.Join(endpointPath, filePath)

	isSubPath, err := utils.IsSubPath(endpointPath, fullPath)
	if err != nil {
		errh.Err(log.ErrUnknown("error checking subpath: " + err.Error()))
		return
	}
	if !isSubPath {
		errh.Err(log.ErrOutOfEndpoint(rawPath, endpoint))
		return
	}

	fileStat, err := os.Stat(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		dir := path.Dir(fullPath)
		if recursive {
			if err = os.MkdirAll(dir, 0777); err != nil {
				errh.Warn(log.ErrUnknown("err making dir: " + err.Error()))
				return
			}
		}
		dirStat, err := os.Stat(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				errh.Warn(log.ErrDirNotExist(dir))
				return
			}
			errh.Err(log.ErrUnknown("err getting dir stat: " + err.Error()))
			return
		}
		if !dirStat.IsDir() {
			errh.Err(log.ErrUnknown("err making dir: " + err.Error()))
			return
		}
	}

	if fileStat.IsDir() {
		errh.Warn(log.ErrPathIsDir(rawPath))
		return
	}
	if !force {
		errh.Warn(log.ErrFileExist(rawPath))
		return
	}

	// TODO maybe use smart transfer like rsync?
	file, err := os.Create(fullPath)
	if err != nil {
		errh.Err(log.ErrUnknown("error creating file: " + err.Error()))
		return
	}

	defer r.Body.Close()
	if _, err = io.Copy(file, r.Body); err != nil {
		errh.Err(log.ErrUnknown("error writing to file: " + err.Error()))
		return
	}

	respJson, err := wrapAPIResponse(map[string]string{})
	if err != nil {
		errh.Err(log.ErrUnknown("error marshaling json: " + err.Error()))
		return
	}
	w.Write(respJson)
}
