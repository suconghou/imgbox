package main

import (
	"flag"
	"fmt"
	_ "ipproxypool/hunter"
	"ipproxypool/proxy"
	"ipproxypool/route"
	"ipproxypool/util"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"
)

var (
	startTime = time.Now()
)

var sysStatus struct {
	Uptime       string
	GoVersion    string
	MemAllocated uint64
	MemTotal     uint64
	MemSys       uint64
	NumGoroutine int
	CPUNum       int
	Pid          int
}

func main() {
	var (
		port int
		host string
		root string
	)
	flag.IntVar(&port, "p", 6060, "listen port")
	flag.StringVar(&host, "h", "", "bind address")
	flag.StringVar(&root, "d", "", "document root")
	flag.Parse()
	if err := serve(host, port, root); err != nil {
		util.Logger.Print(err)
	}
}

func serve(host string, port int, root string) error {
	if root != "" {
		http.Handle("/public/", http.StripPrefix("/public", http.FileServer(http.Dir(root))))
	}
	http.HandleFunc("/status", status)
	http.HandleFunc("/", routeMatch)
	util.Logger.Printf("Starting up on port %d", port)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil)
}

func status(w http.ResponseWriter, r *http.Request) {
	memStat := new(runtime.MemStats)
	runtime.ReadMemStats(memStat)
	sysStatus.Uptime = time.Since(startTime).String()
	sysStatus.NumGoroutine = runtime.NumGoroutine()
	sysStatus.MemAllocated = memStat.Alloc
	sysStatus.MemTotal = memStat.TotalAlloc
	sysStatus.MemSys = memStat.Sys
	sysStatus.CPUNum = runtime.NumCPU()
	sysStatus.GoVersion = runtime.Version()
	sysStatus.Pid = os.Getpid()
	util.JSONPut(w, sysStatus)
}

func routeMatch(w http.ResponseWriter, r *http.Request) {
	found := false
	for _, p := range route.Route {
		if p.Reg.MatchString(r.URL.Path) {
			found = true
			if err := p.Handler(w, r, p.Reg.FindStringSubmatch(r.URL.Path)); err != nil {
				util.Logger.Print(err)
			}
			break
		}
	}
	if !found {
		fallback(w, r)
	}
}

func fallback(w http.ResponseWriter, r *http.Request) {
	const index = "index.html"
	files := []string{index}
	if r.URL.Path != "/" {
		files = []string{r.URL.Path, path.Join(r.URL.Path, index)}
	}
	if !tryFiles(files, w, r) {
		if util.ValidProxyURL(r.RequestURI) {
			proxy.URL(w, r)
		} else {
			http.NotFound(w, r)
		}
	}
}

func tryFiles(files []string, w http.ResponseWriter, r *http.Request) bool {
	for _, file := range files {
		realpath := filepath.Join("./static", file)
		if f, err := os.Stat(realpath); err == nil {
			if f.Mode().IsRegular() {
				http.ServeFile(w, r, realpath)
				return true
			}
		}
	}
	return false
}
