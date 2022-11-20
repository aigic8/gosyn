package server

import (
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/gorilla/mux"
)

type fileHandler struct {
	Endpoints   map[string]string
	MaxHashSize int64
}

func (fHandler *fileHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileVar := strings.TrimSpace(vars["file"])
	if fileVar == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("file is empty"))
		log.Printf("no file var spicified in request\n")
		return
	}

	endpoint, filePath, err := splitEndpointAndFile(fileVar)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("sent a bad file descriptor"))
		log.Printf("error spliting endpoint and filePath: %v\n", err)
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("endpoint not found"))
		log.Printf("endpoint '%s' not found\n", endpoint)
		return
	}

	fullPath := path.Join(endpointPath, filePath)

	stat, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("file not found"))
			log.Printf("file '%s' not exist:\n", fullPath)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error happened"))
		log.Printf("error stating '%s': %v\n", fullPath, err)
		return
	}

	if stat.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("path is a dir"))
		log.Printf("tried to download path '%s' which is a dir\n", fullPath)
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("there is an error with file"))
		log.Printf("error opening file '%s': %v\n", fullPath, err)
	}

	// TODO use brotli or gzip for text files
	io.Copy(w, file)
}

// TODO is it useful?
func (fHandler *fileHandler) GetHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileVar := strings.TrimSpace(vars["file"])

	if fileVar == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("empty/no file parameter"))
		log.Printf("empty/no file parameter\n")
		return
	}

	endpoint, filePath, err := splitEndpointAndFile(fileVar)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("sent a bad file descriptor"))
		log.Printf("error spliting endpoint and filePath: %v\n", err)
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("endpoint not found"))
		log.Printf("endpoint '%s' not found\n", endpoint)
		return
	}

	fullPath := path.Join(endpointPath, filePath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("file not found"))
			log.Printf("file '%s' not found\n", fullPath)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error happened!"))
		log.Printf("error stating file '%s': %v\n", fullPath, err)
		return
	}

	if fileInfo.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error with file"))
		log.Printf("wanted to get hash of dir '%s'", fullPath)
		return
	}

	if fHandler.MaxHashSize > 0 && fileInfo.Size() > fHandler.MaxHashSize {
		w.WriteHeader(http.StatusBadRequest) // TODO maybe a better status code?
		w.Write([]byte("file is bigger than max hash size"))
		return
	}

	// TODO add option to hash file smartly like rsync
	hash, err := hashFile(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error happened!"))
		log.Printf("error hashing file '%s': %v", fullPath, err)
		return
	}

	// FIXME use json!!
	w.Write([]byte(hash))
}

func (fHandler *fileHandler) AddNew(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.Header.Get("x-file-path"))

	rawRecursive := strings.TrimSpace(r.Header.Get("x-recursive"))
	recursive := rawRecursive == "true"

	rawForce := strings.TrimSpace(r.Header.Get("x-force"))
	force := rawForce == "true"

	if rawPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("file path is not specified"))
		log.Print("got request without x-file-path header")
		return
	}

	endpoint, filePath, err := splitEndpointAndFile(rawPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("file path is not valid"))
		log.Printf("error parsing path: %v\n", err)
		return
	}

	endpointPath, endpointExists := fHandler.Endpoints[endpoint]
	if !endpointExists {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("endpoint not found"))
		log.Printf("endpoint '%s' not found\n", endpoint)
		return
	}

	// FIXME !!!!IMPORTANT!!! find a way to filter paths using '..' to go out of endpoint
	fullPath := path.Join(endpointPath, filePath)

	fileStat, err := os.Stat(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		dir := path.Dir(fullPath)
		if recursive {
			if err = os.MkdirAll(dir, 0777); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal server error happened"))
				log.Printf("error making dir '%s': %v\n", dir, err)
				return
			}
		}
		dirStat, err := os.Stat(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("base directory does not exist"))
				log.Printf("dir '%s' does not eixst\n", dir)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error happened"))
			log.Printf("error stating path '%s': %v\n", dir, err)
			return
		}
		if !dirStat.IsDir() {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("base path is not a dir"))
			log.Printf("path '%s' is not a dir:\n", dir)
			return
		}
	}

	if fileStat.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("dir with same path exist"))
		log.Printf("can't create file '%s' beacuase same dir exists\n", fullPath)
		return
	}
	if !force {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("file already exists"))
		log.Printf("can't create file '%s' beacuase same file exists\n", fullPath)
		return
	}

	// TODO maybe use smart transfer like rsync?
	file, err := os.Create(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("internal server error"))
		log.Printf("can't create file '%s': %v\n", fullPath, err)
		return
	}

	defer r.Body.Close()
	if _, err = io.Copy(file, r.Body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("internal server error"))
		log.Printf("couldnt write file data '%s': %v\n", fullPath, err)
		return
	}

	// FIXME use json!!!!!
	w.Write([]byte("OK"))
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
