package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	log "github.com/meifamily/logrus"
	"github.com/robfig/cron"

	ctrlr "github.com/liam-lai/ptt-alertor/controllers"
	"github.com/liam-lai/ptt-alertor/jobs"
	"github.com/liam-lai/ptt-alertor/line"
	"github.com/liam-lai/ptt-alertor/messenger"
	"github.com/liam-lai/ptt-alertor/myutil"
)

var auth map[string]string

type myRouter struct {
	httprouter.Router
}

func (mr myRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"IP":  r.RemoteAddr,
		"URI": r.URL.Path,
	}).Info("visit")
	mr.Router.ServeHTTP(w, r)
}

func newRouter() *myRouter {
	return &myRouter{
		Router: *httprouter.New(),
	}
}

func basicAuth(handle httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		user, password, hasAuth := r.BasicAuth()
		if hasAuth && user == auth["user"] && password == auth["password"] {
			handle(w, r, params)
		} else {
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}

func main() {
	log.Info("Start Jobs")
	startJobs()
	// Web Server
	log.Info("Web Server Start on Port 9090")

	router := newRouter()
	router.NotFound = http.FileServer(http.Dir("public"))
	m := messenger.New()

	router.GET("/", ctrlr.Index)
	router.GET("/messenger", ctrlr.MessengerIndex)
	router.GET("/line", ctrlr.LineIndex)
	router.GET("/redirect/:sha1", ctrlr.Redirect)
	router.GET("/top", ctrlr.Top)

	router.POST("/broadcast", basicAuth(ctrlr.Broadcast))

	// boards apis
	router.GET("/boards/:boardName/articles/:code", ctrlr.BoardArticle)
	router.GET("/boards/:boardName/articles", ctrlr.BoardArticleIndex)
	router.GET("/boards", ctrlr.BoardIndex)

	// users apis
	router.GET("/users/:account", basicAuth(ctrlr.UserFind))
	router.GET("/users", basicAuth(ctrlr.UserAll))
	router.POST("/users", basicAuth(ctrlr.UserCreate))
	router.PUT("/users/:account", basicAuth(ctrlr.UserModify))

	// line
	router.POST("/line/callback", line.HandleRequest)
	router.POST("/line/notify/callback", line.CatchCallback)

	// facebook messenger
	router.GET("/messenger/webhook", m.Verify)
	router.POST("/messenger/webhook", m.Received)

	err := http.ListenAndServe(":9090", router)
	if err != nil {
		log.Fatal("ListenAndServer ", err)
	}

}

func startJobs() {
	go new(jobs.Checker).Run()
	go jobs.NewPushChecker().Run()
	c := cron.New()
	c.AddJob("@every 1h", jobs.NewGenerator())
	c.AddJob("@every 1h", jobs.NewTop())
	c.Start()
}

func init() {
	auth = myutil.Config("auth")
	new(jobs.CleanUpBoards).Run()
	jobs.NewGenerator().Run()
	jobs.NewFetcher().Run()
	jobs.NewTop().Run()
}
