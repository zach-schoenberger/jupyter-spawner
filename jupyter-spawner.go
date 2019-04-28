package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"html/template"
	"runtime"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func dumpErr(e error, depth int) {
	log.Errorf("%s\n", e.Error())
	err, ok := e.(stackTracer)
	if !ok {
		//panic("oops, err does not implement stackTracer")
		return
	}

	st := err.StackTrace()
	if depth == 0 {
		depth = len(st) - 1
	}
	log.Errorf("%+v", st[0:depth])
}

var runCache RunCache
var redisConfig *RedisConfig
var k8Client *K8Client
var log = logrus.New()
var lw = log.Writer()

var jobTemplateFile = "./job.tmpl"
var jobTemplate *template.Template
var jobConfig *JobConfig

func main() {
	runtime.GOMAXPROCS(4)
	defer func() {
		if err := lw.Close(); err != nil {
			panic(fmt.Errorf("Failed to close file: %s \n", err))
		}
	}()

	log.SetLevel(logrus.DebugLevel)
	viper.SetConfigName("jspawner") // name of appConfig file (without extension)
	viper.AddConfigPath(".")
	//viper.AutomaticEnv()
	//_ = viper.BindEnv("REDISPASSWORD")

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

	jobConfig = new(JobConfig)
	if err := getConfig("job", jobConfig); err != nil {
		panic(err)
	}

	g := gin.New()
	g.Use(gin.LoggerWithWriter(lw), gin.Recovery())
	g.POST("/notebook/run", postRunNotebook)
	g.POST("/notebook/end/:requestId", postNotebookResult)

	if err := g.Run(fmt.Sprintf(":%d", appConfig.Port)); err != nil {
		_ = fmt.Errorf(err.Error())
	}
}

func postRunNotebook(c *gin.Context) {
	status, resp := processRunNotebook(c)
	log.Infof("%d: %+v", status, resp)
	c.JSON(status, resp)
}

func postNotebookResult(c *gin.Context) {
	status, resp := postResult(c)
	log.Infof("%d: %+v", status, resp)
	c.JSON(status, resp)
}
