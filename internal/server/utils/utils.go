package utils

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type Logger struct {
	Logger *zap.SugaredLogger
}

func NewLogger() (*Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{Logger: logger.Sugar()}, nil
}

type (
	APIERRHandler struct {
		L *Logger
		R *http.Request
		W http.ResponseWriter
	}

	HTTPErrResponse struct {
		OK  bool   `json:"ok"` // always false
		Msg string `json:"message"`
	}
)

func NewAPIErrHandler(l *Logger, r *http.Request, w http.ResponseWriter) *APIERRHandler {
	return &APIERRHandler{R: r, W: w, L: l}
}

func (eHandler *APIERRHandler) Err(err HTTPErr) {
	status := err.Status()
	eHandler.W.WriteHeader(status)
	httpErr := HTTPErrResponse{OK: false, Msg: err.RespMsg()}
	jsonData, _ := json.Marshal(&httpErr) // TODO can throw err?
	eHandler.W.Write(jsonData)
	eHandler.L.Logger.Errorw(err.LogMsg(), "status", status, "url", eHandler.R.URL.String())
}

func (eHandler *APIERRHandler) Warn(err HTTPErr) {
	status := err.Status()
	eHandler.W.WriteHeader(status)
	httpErr := HTTPErrResponse{OK: false, Msg: err.RespMsg()}
	jsonData, _ := json.Marshal(&httpErr) // TODO can throw err?
	eHandler.W.Write(jsonData)
	eHandler.L.Logger.Warnw(err.LogMsg(), "status", status, "url", eHandler.R.URL.String())
}

type HTTPErr interface {
	RespMsg() string
	LogMsg() string
	Status() int
}

type BasicHTTPErr struct {
	respMsg string
	logMsg  string
	status  int
}

func (err *BasicHTTPErr) Status() int {
	return err.status
}

func (err *BasicHTTPErr) LogMsg() string {
	return err.logMsg
}

func (err *BasicHTTPErr) RespMsg() string {
	return err.respMsg
}

func (err *BasicHTTPErr) Error() string {
	return err.logMsg
}

func ErrVarNotFound(varName string) HTTPErr {
	return &BasicHTTPErr{
		status:  http.StatusBadRequest,
		respMsg: varName + " is not empty",
		logMsg:  "var '" + varName + "' is empty",
	}
}

func ErrFileNotFound(addr string) HTTPErr {
	msg := "file '" + addr + "' not found"
	return &BasicHTTPErr{
		status:  http.StatusNotFound,
		respMsg: msg,
		logMsg:  msg,
	}
}

func ErrBadFileDesc(addr string, err error) HTTPErr {
	return &BasicHTTPErr{
		status:  http.StatusBadRequest,
		respMsg: "bad file descriptor '" + addr + "'",
		logMsg:  "error parsing file path: " + err.Error(),
	}
}

func ErrEndpointNotFound(endpoint string) HTTPErr {
	msg := "endpoint '" + endpoint + "' not found"
	return &BasicHTTPErr{
		status:  http.StatusNotFound,
		respMsg: msg,
		logMsg:  msg,
	}
}

func ErrUnknown(err string) HTTPErr {
	return &BasicHTTPErr{
		status:  http.StatusInternalServerError,
		respMsg: "unknown error :(",
		logMsg:  err,
	}
}

func ErrPathIsDir(dirPath string) HTTPErr {
	msg := "path '" + dirPath + "' is a directory"
	return &BasicHTTPErr{
		status:  http.StatusBadRequest,
		respMsg: msg,
		logMsg:  msg,
	}
}

func ErrFileTooBigToHash(filePath string, maxHashSize int64) HTTPErr {
	msg := fmt.Sprintf("file '%s' is larger than max hash size '%d'", filePath, maxHashSize)
	return &BasicHTTPErr{
		status:  http.StatusBadRequest,
		respMsg: msg,
		logMsg:  msg,
	}
}

func ErrHeaderNotFound(valueName string, headerName string) HTTPErr {
	return &BasicHTTPErr{
		status:  http.StatusBadRequest,
		respMsg: "'" + valueName + "' is not defind",
		logMsg:  "'" + headerName + "' is not sent in request",
	}
}

func ErrDirNotExist(dirPath string) HTTPErr {
	msg := "directory '" + dirPath + "' does not exist"
	return &BasicHTTPErr{
		status:  http.StatusBadRequest,
		respMsg: msg,
		logMsg:  msg,
	}
}

func ErrFileExist(filePath string) HTTPErr {
	msg := "a file with same path '" + filePath + "' exists"
	return &BasicHTTPErr{
		status:  http.StatusBadRequest,
		respMsg: msg,
		logMsg:  msg,
	}
}
