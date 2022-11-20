package server

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

const DEFAULT_MAX_HASH_SIZE int64 = 50 * 1024 * 1024 // 50 MB

type Server struct {
	address     string
	endpoints   map[string]string
	MaxHashSize int64
}

// TODO response with ok true and false
// type Response[T any] struct {
// 	Ok   bool   `json:"ok"`
// 	Err  string `json:"error"`
// 	Data T      `json:"data"`
// }

// func errResponse(message string) *Response[map[bool]bool] {
// 	return &Response[map[bool]bool]{
// 		Ok:   true,
// 		Err:  message,
// 		Data: map[bool]bool{},
// 	}
// }

func NewServer(addr string, endpoints map[string]string) *Server {
	return &Server{
		address:     addr,
		MaxHashSize: DEFAULT_MAX_HASH_SIZE,
		endpoints:   endpoints,
	}
}

func (server *Server) Start() error {
	router := server.makeRoutes()

	// TODO use quic and http2
	srv := &http.Server{
		Handler:      router,
		Addr:         server.address,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv.ListenAndServe()
}

func (server *Server) makeRoutes() *mux.Router {
	r := mux.NewRouter()

	eHandler := endpointHanlder{Endpoints: server.endpoints}
	r.HandleFunc("endpoints/{endpoint}", eHandler.Get).Methods(http.MethodGet)
	r.HandleFunc("endpoints/list", eHandler.GetAll).Methods(http.MethodGet)

	fHandler := fileHandler{Endpoints: server.endpoints, MaxHashSize: server.MaxHashSize}
	r.HandleFunc("files/{file}/hash", fHandler.GetHash).Methods(http.MethodGet)
	r.HandleFunc("files/{file}", fHandler.Get).Methods(http.MethodGet)
	r.HandleFunc("files/new", fHandler.AddNew).Methods(http.MethodPut)
	// TODO r.HandleFunc("files/{file}/smart", fHandler.SmartGet).Methods.(http.MethodGet)

	return r
}
