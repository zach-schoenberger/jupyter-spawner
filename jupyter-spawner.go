package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"html/template"
	"io/ioutil"
	"net/http"
	"runtime"
)

type QueryParams struct {
	UserId          string `form:"uid" binding:"required"`
	UserAddress     string `form:"adr" binding:"required"`
	UserAddressPort int    `form:"prt" binding:"required"`
	Force           bool   `form:"frc"`
}

type PostData struct {
	PythonScript []byte `form:"pyscript" binding:"required"`
}

type RunNotebookResponse struct {
	RequestId    string  `json:"requestId"`
	PyscriptHash string  `json:"pyscriptHash"`
	Status       *string `json:"status,omitempty"`
	Error        *string `json:"error,omitempty"`
	Result       *string `json:"result,omitempty"`
}

type TemplateData struct {
	JobName      string
	Image        string
	PyScriptHash string
	UserId       string
	RequestId    string
}

var ERROR_STATUS = "ERROR"
var RUNNING_STATUS = "RUNNING"
var FINISHED_STATUS = "FINISHED"

var runCache RunCache
var redisConfig *RedisConfig
var k8Client *K8Client
var l = logrus.New()
var lw = l.Writer()

var imageName = "jhub-spawner/tester:1.0"
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
	//g.POST("/notebook/result/:requestId", postRunNotebook)

	if err := g.Run(fmt.Sprintf(":%d", appConfig.Port)); err != nil {
		_ = fmt.Errorf(err.Error())
	}
}

func postRunNotebook(c *gin.Context) {
	c.JSON(processRunNotebook(c))
}

func processRunNotebook(c *gin.Context) (int, RunNotebookResponse) {
	requestId := uuid.NewUUID()
	queryParams := new(QueryParams)
	if err := c.Bind(queryParams); err != nil {
		e := err.Error()
		return http.StatusBadRequest, RunNotebookResponse{
			RequestId: requestId.String(),
			Error:     &e,
			Status:    &ERROR_STATUS}
	}

	postData, err := getPostData(c)
	if err != nil {
		e := err.Error()
		return http.StatusBadRequest, RunNotebookResponse{
			RequestId: requestId.String(),
			Error:     &e,
			Status:    &ERROR_STATUS}
	}

	sha := sha256.Sum256(postData.PythonScript)
	postHash := fmt.Sprintf("%x", sha)

	isRunNotebook, err := runCache.Set(postHash, requestId.String())
	if err != nil {
		serverError := fmt.Errorf("Failed to write to cache: %s\n", err)
		l.Errorln(serverError)
		e := serverError.Error()
		return http.StatusInternalServerError, RunNotebookResponse{
			RequestId:    requestId.String(),
			PyscriptHash: postHash,
			Error:        &e,
			Status:       &ERROR_STATUS}
	}

	if isRunNotebook || queryParams.Force == true {
		_, err := runNotebook(queryParams, requestId.String(), postHash, postData)
		if err != nil {
			serverError := fmt.Errorf("Failed to run notebook: %s\n", err)
			l.Errorln(serverError.Error())
			e := serverError.Error()
			return http.StatusInternalServerError, RunNotebookResponse{
				RequestId:    requestId.String(),
				PyscriptHash: postHash,
				Error:        &e,
				Status:       &ERROR_STATUS}
		}
		return http.StatusOK, RunNotebookResponse{
			RequestId:    requestId.String(),
			PyscriptHash: postHash,
			Status:       &RUNNING_STATUS}
	} else {
		if result, err := runCache.Get(postHash); err != nil {
			serverError := fmt.Errorf("Failed to read from cache: %s\n", err)
			l.Errorln(serverError.Error())
			e := serverError.Error()
			return http.StatusInternalServerError, RunNotebookResponse{
				RequestId:    requestId.String(),
				PyscriptHash: postHash,
				Error:        &e,
				Status:       &ERROR_STATUS}
		} else {
			return http.StatusOK, RunNotebookResponse{
				RequestId:    requestId.String(),
				PyscriptHash: postHash,
				Status:       &FINISHED_STATUS,
				Result:       &result}
		}
	}
}

func getPostData(c *gin.Context) (*PostData, error) {
	postData := new(PostData)
	if fh, err := c.FormFile("pyscript"); err != nil || fh == nil {
		if c.Bind(postData) != nil {
			return nil, fmt.Errorf("missing pyscript")
		}
	} else {
		if f, err := fh.Open(); err != nil {
			return nil, fmt.Errorf("could not read pyscript")
		} else {
			postData.PythonScript, _ = ioutil.ReadAll(f)
		}
	}
	return postData, nil
}

func runNotebook(params *QueryParams, requestId string, pyScriptHash string, data *PostData) (string, error) {
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
		return "", err
	}
	l.Debugf("Job definition: %s\n", buf)
	configMap := make([]ConfigMapFile, 2)
	configMap[0] = ConfigMapFile{Name: "pyScript.py", Data: data.PythonScript}
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	configMap[1] = ConfigMapFile{Name: "params.json", Data: paramBytes}

	if _, err := k8Client.PutConfigMap(pyScriptHash, configMap); err != nil {
		return "", err
	}
	if _, err := k8Client.StartJob(buf.Bytes()); err != nil {
		return "", err
	}
	return "", nil
}
