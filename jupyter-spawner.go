package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"html/template"
	"net/http"
	"runtime"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func dumpErr(e error, depth int) {
	l.Errorf("%s\n", e.Error())
	err, ok := e.(stackTracer)
	if !ok {
		panic("oops, err does not implement stackTracer")
	}

	st := err.StackTrace()
	if depth == 0 {
		depth = len(st) - 1
	}
	l.Errorf("%+v", st[0:depth])
}

var runCache RunCache
var redisConfig *RedisConfig
var k8Client *K8Client
var l = logrus.New()
var lw = l.Writer()

var imageName = "zschoenb/jhub-tester:1.0"
var jobTemplateFile = "./job.tmpl"
var jobTemplate *template.Template

func main() {
	runtime.GOMAXPROCS(4)
	defer func() {
		if err := lw.Close(); err != nil {
			panic(fmt.Errorf("Failed to close file: %s \n", err))
		}
	}()

	l.SetLevel(logrus.DebugLevel)
	viper.SetConfigName("jspawner") // name of appConfig file (without extension)
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("Fatal error appConfig file: %s \n", err))
	}

	appConfig := new(AppConfig)
	if err := getConfig("app", appConfig); err != nil {
		panic(err)
	}
	gin.SetMode(appConfig.Mode)

	if jt, err := template.ParseFiles(jobTemplateFile); err != nil {
		panic(err)
	} else {
		jobTemplate = jt
	}

	k8Client = ConnectToK8()

	redisConfig = new(RedisConfig)
	if err := getConfig("redis", redisConfig); err != nil {
		panic(err)
	}

	runCache = NewRedisCache(*redisConfig)

	g := gin.New()
	g.Use(gin.LoggerWithWriter(lw), gin.Recovery())
	g.POST("/notebook/run", postRunNotebook)
	g.POST("/notebook/end/:requestId", postNotebookResult)

	if err := g.Run(fmt.Sprintf(":%d", appConfig.Port)); err != nil {
		_ = fmt.Errorf(err.Error())
	}
}

func postRunNotebook(c *gin.Context) {
	c.JSON(processRunNotebook(c))
}

func postNotebookResult(c *gin.Context) {
	c.JSON(postResult(c))
}

func postResult(c *gin.Context) (int, ResultResponse) {
	resultRequest := new(ResultRequest)
	if err := c.ShouldBindUri(resultRequest); err != nil {
		e := err.Error()
		return http.StatusBadRequest, ResultResponse{
			Error:  &e,
			Status: ErrorStatus}
	}

	postData, err := getPostData(c)
	if err != nil {
		e := err.Error()
		return http.StatusBadRequest, ResultResponse{
			Error:  &e,
			Status: ErrorStatus}
	}

	if _, err := runCache.Set(resultRequest.RequestId, string(postData.PythonScript)); err != nil {
		serverError := errors.Wrap(err, "Failed to write to cache")
		dumpErr(serverError, 0)
	}

	return http.StatusOK, ResultResponse{
		Status: FinishedStatus,
	}
}

func getRunResults(runRequest *RunNotebookRequest) (string, string, error) {
	var requestId, data string
	var err error
	if requestId, err = runCache.Get(runRequest.PostHash); err != nil {
		return "", "", errors.WithStack(err)
	}
	if data, err = runCache.Get(requestId); err != nil {
		return "", "", errors.WithStack(err)
	}
	return requestId, data, nil
}
