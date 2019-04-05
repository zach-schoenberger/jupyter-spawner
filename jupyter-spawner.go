package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"html/template"
	"io/ioutil"
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

func getSha(data *PostData) string {
	sha := sha256.Sum256(data.PythonScript)
	postHash := fmt.Sprintf("%x", sha)
	return postHash
}

func postRunNotebook(c *gin.Context) {
	c.JSON(processRunNotebook(c))
}

func postNotebookResult(c *gin.Context) {
	c.JSON(postResult(c))
}

func getRunRequest(c *gin.Context, requestId string) (*RunNotebookRequest, error) {
	runRequest := RunNotebookRequest{nil, nil, "", ""}
	qp := QueryParams{}
	runRequest.QueryParams = &qp
	if err := c.BindQuery(&qp); err != nil {
		return nil, errors.WithStack(err)
	}

	postData, err := getPostData(c)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	postData.PythonScript, err = processScript(requestId, postData.PythonScript)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	runRequest.PostData = postData
	runRequest.PostHash = getSha(postData)
	return &runRequest, nil
}

func processRunNotebook(c *gin.Context) (int, RunNotebookResponse) {
	requestId := uuid.NewUUID().String()
	runRequest, err := getRunRequest(c, requestId)
	if err != nil {
		e := err.Error()
		l.Errorln(e)
		return http.StatusBadRequest, RunNotebookResponse{
			RequestId: requestId,
			Error:     &e,
			Status:    ErrorStatus.pointer()}
	}

	isRunNotebook := validateRunRequest(requestId, runRequest)

	if isRunNotebook {
		if err := runNotebook(runRequest.QueryParams, requestId, runRequest.PostHash, runRequest.PostData); err != nil {
			dumpErr(err, 0)
			e := err.Error()
			return http.StatusInternalServerError, RunNotebookResponse{
				RequestId:    requestId,
				PyscriptHash: runRequest.PostHash,
				Error:        &e,
				Status:       ErrorStatus.pointer()}
		}
		return http.StatusOK, RunNotebookResponse{
			RequestId:    requestId,
			PyscriptHash: runRequest.PostHash,
			Status:       RunningStatus.pointer()}
	} else {
		if runRequestId, result, err := getRunResults(runRequest); err != nil {
			dumpErr(err, 0)
			e := err.Error()
			return http.StatusInternalServerError, RunNotebookResponse{
				RequestId:    requestId,
				PyscriptHash: runRequest.PostHash,
				Error:        &e,
				Status:       ErrorStatus.pointer()}
		} else {
			return http.StatusOK, RunNotebookResponse{
				RequestId:    runRequestId,
				PyscriptHash: runRequest.PostHash,
				Status:       FinishedStatus.pointer(),
				Result:       &result}
		}
	}
}

func getPostData(c *gin.Context) (*PostData, error) {
	postData := new(PostData)
	rb, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	postData.PythonScript = rb
	return postData, nil
}

func runNotebook(params *QueryParams, requestId string, pyScriptHash string, data *PostData) error {
	//var buf = bytes.NewBuffer(make([]byte, 4096))
	buf := &bytes.Buffer{}
	var templateData = TemplateData{
		JobName:      requestId,
		Image:        imageName,
		PyScriptHash: pyScriptHash,
		UserId:       params.UserId,
		RequestId:    requestId,
	}
	if err := jobTemplate.Execute(buf, templateData); err != nil {
		return errors.WithStack(err)
	}
	l.Debugf("Job definition: %s\n", buf)
	configMap := make([]ConfigMapFile, 2)
	configMap[0] = ConfigMapFile{Name: "pyScript.pyc", Data: data.PythonScript}
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return errors.WithStack(err)
	}
	configMap[1] = ConfigMapFile{Name: "params.json", Data: paramBytes}

	if _, err := k8Client.PutConfigMap(pyScriptHash, configMap); err != nil {
		return errors.WithStack(err)
	}
	if _, err := k8Client.StartJob(buf.Bytes()); err != nil {
		return err
	}
	return nil
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

func validateRunRequest(requestId string, runRequest *RunNotebookRequest) bool {
	if !runRequest.Force {
		_, err := runCache.Get(runRequest.PostHash)
		if err != nil {
			if err != redis.Nil {
				serverError := errors.Wrap(err, "Failed to read from cache")
				dumpErr(serverError, 0)
				return false
			}
			return addRunToCache(requestId, runRequest)
		}
		return false
	} else {
		return addRunToCache(requestId, runRequest)
	}
}

func addRunToCache(requestId string, runRequest *RunNotebookRequest) bool {
	_, err := runCache.Set(runRequest.PostHash, requestId)
	if err != nil {
		serverError := errors.Wrap(err, "Failed to write to cache")
		dumpErr(serverError, 0)
		return false
	}

	if _, err := runCache.Set(requestId, string(RunningStatus)); err != nil {
		serverError := errors.Wrap(err, "Failed to write to cache")
		dumpErr(serverError, 0)
		return false
	}
	return true
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
