package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/yookoala/gofast"
)

var (
	revProxURL     string
	listenHostPort string
	sockAddress    string
	phpFsPath      string

	revProxHost string

	hostProxy = make(map[string]*httputil.ReverseProxy)

	staticFileExt = make(map[string]int)
)

type baseHandle struct{}

func phpFast() {

	connFactory := gofast.SimpleConnFactory("unix", sockAddress)

	gs := gofast.NewHandler(
		gofast.NewPHPFS(phpFsPath)(gofast.BasicSession),
		gofast.SimpleClientFactory(connFactory),
	)
	// route all requests to relevant PHP file
	http.Handle("/", gs)

	err := http.ListenAndServe(revProxHost, nil)
	if err != nil {
		panic(err)
	}
}

func headerToJson(header http.Header) {

	jsonHeader, err := json.Marshal(header)
	if err != nil {
		fmt.Println(err)
	}

	log.Println(string(jsonHeader))

}

func startRevProx() {
	revMux := http.NewServeMux()

	h := &baseHandle{}

	revMux.Handle("/", h)

	server := &http.Server{
		Addr:    listenHostPort,
		Handler: h,
	}

	log.Printf("Starting server listening on %s", listenHostPort)

	err := server.ListenAndServe()

	if err != nil {
		panic(err)
	}
}

func (h *baseHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Server", "WP GoFast")

	rip := r.Header.Get("X-Forwarded-For")
	if rip == "" {
		var err error
		rip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			log.Println(err)
		}
	}

	fpExt := filepath.Ext(r.URL.Path)

	r.Header.Add("URL", r.URL.String())

	headerToJson(r.Header)

	if fpExt != ".php" && fpExt != "" {
		// serve a static file here
		http.ServeFile(w, r, phpFsPath+"/"+r.URL.Path)
	} else {
		// pass to goFast
		remoteURL, err := url.Parse(revProxURL)
		if err != nil {
			log.Println(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(remoteURL)

		proxy.ServeHTTP(w, r)
	}

}

func loadConfig() {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	revProxURL = viper.GetString("revProxURL")
	listenHostPort = viper.GetString("listenHostPort")
	sockAddress = viper.GetString("sockAddress")
	phpFsPath = viper.GetString("phpFsPath")

	// parse the host from the revProxURL
	u, err := url.Parse(revProxURL)
	if err != nil {
		log.Fatal(err)
	}

	revProxHost = u.Host

}

func main() {

	loadConfig()
	go phpFast()
	startRevProx()

}
