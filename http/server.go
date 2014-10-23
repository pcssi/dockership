package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	s := &server{}
	s.readConfig()
	s.configure()
	s.configureAuth()
	s.run()
}

type server struct {
	mux    *mux.Router
	oauth  *OAuth
	config config
}

func (s *server) configure() {
	s.mux = mux.NewRouter()

	// status
	s.mux.Path("/rest/status").Methods("GET").HandlerFunc(s.HandleStatus)
	s.mux.Path("/rest/status/{project:.*}").Methods("GET").HandlerFunc(s.HandleStatus)

	// containers
	s.mux.Path("/rest/containers").Methods("GET").HandlerFunc(s.HandleContainers)
	s.mux.Path("/rest/containers/{project:.*}").Methods("GET").HandlerFunc(s.HandleContainers)

	// deploy
	s.mux.Path("/rest/deploy/{project:.*}/{enviroment:.*}").Methods("GET").HandlerFunc(s.HandleDeploy)

	// logged-user
	s.mux.Path("/rest/user").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := s.oauth.getUser(s.oauth.getToken(r))
		s.json(w, 200, user)
	})

	// assets
	s.mux.Path("/").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		content, _ := Asset("static/index.html")
		w.Write(content)
	})

	s.mux.Path("/app.js").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/app.js")
		w.Header().Set("Content-Type", "application/javascript")
		content, _ := Asset("static/app.js")
		w.Write(content)
	})
}

func (s *server) configureAuth() {
	s.oauth = NewOAuth(&s.config)
}

func (s *server) readConfig() {
	if err := s.config.LoadFile("config.ini"); err != nil {
		panic(err)
	}
}

func (s *server) run() {
	if err := http.ListenAndServe(s.config.HTTP.Listen, s); err != nil {
		panic(err)
	}
}

func (s *server) json(w http.ResponseWriter, code int, response interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.Encode(response)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.oauth.Handler(w, r) {
		s.mux.ServeHTTP(w, r)
	}
}

// AutoFlusherWrite
type AutoFlusherWriter struct {
	writer     http.ResponseWriter
	autoFlush  *time.Ticker
	closeChan  chan bool
	closedChan chan bool
}

func NewAutoFlusherWriter(writer http.ResponseWriter, duration time.Duration) *AutoFlusherWriter {
	a := &AutoFlusherWriter{
		writer:     writer,
		autoFlush:  time.NewTicker(duration),
		closeChan:  make(chan bool),
		closedChan: make(chan bool),
	}

	go a.loop()
	return a
}

func (a *AutoFlusherWriter) loop() {
	for {
		select {
		case <-a.autoFlush.C:
			a.writer.(http.Flusher).Flush()
		case <-a.closeChan:
			a.writer.(http.Flusher).Flush()
			close(a.closedChan)
			return
		}
	}
}

func (a *AutoFlusherWriter) Write(p []byte) (int, error) {
	return a.writer.Write(p)
}

func (a *AutoFlusherWriter) Close() {
	for {
		select {
		case a.closeChan <- true:
		case <-a.closedChan:
			return
		}
	}
}
