package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"net/http"
)

func processRunNotebook(c *gin.Context) (int, RunNotebookResponse) {
	requestId := uuid.NewUUID().String()
	runRequest, err := getRunRequest(c, requestId)
	if err != nil {
		e := err.Error()
		dumpErr(err, 0)
		return http.StatusBadRequest, RunNotebookResponse{
			RequestId: requestId,
			Error:     &e,
			Status:    ErrorStatus.pointer()}
	}

	isRunNotebook := validateRunRequest(requestId, runRequest)

	if isRunNotebook {
		if err := runNotebook(runRequest.RunQueryParams, requestId, runRequest.PostHash, runRequest.PostData); err != nil {
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
		if runRequestId, status, result, err := getRunResults(runRequest); err != nil {
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
				Status:       &status,
				Result:       &result}
		}
	}
}

func getRunRequest(c *gin.Context, requestId string) (*RunNotebookRequest, error) {
	runRequest := RunNotebookRequest{nil, nil, "", ""}
	qp := RunQueryParams{}
	runRequest.RunQueryParams = &qp
	if err := c.BindQuery(&qp); err != nil {
		return nil, errors.WithStack(err)
	}

	postData, err := getPostData(c)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	postData.PythonScript, err = processScript(requestId, postData.PythonScript)
	if err != nil {
		return nil, err
	}

	runRequest.PostData = postData
	runRequest.PostHash = getSha(postData)
	return &runRequest, nil
}

func runNotebook(params *RunQueryParams, requestId string, pyScriptHash string, data *PostData) error {
	//var buf = bytes.NewBuffer(make([]byte, 4096))
	buf := &bytes.Buffer{}
	var templateData = TemplateData{
		JobName:      requestId,
		Image:        jobConfig.Image,
		PyScriptHash: pyScriptHash,
		UserId:       params.UserId,
		RequestId:    requestId,
	}
	if err := jobTemplate.Execute(buf, templateData); err != nil {
		return errors.WithStack(err)
	}
	log.Debugf("Job definition: %s\n", buf)
	configMap := make([]ConfigMapFile, 2)
	configMap[0] = ConfigMapFile{Name: "pyScript.py", Data: data.PythonScript}
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return errors.WithStack(err)
	}
	configMap[1] = ConfigMapFile{Name: "params.json", Data: paramBytes}

	if _, err := k8Client.PutConfigMap(pyScriptHash, configMap); err != nil {
		return err
	}
	if _, err := k8Client.StartJob(buf.Bytes()); err != nil {
		return err
	}
	return nil
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
	var ret = true
	qp, _ := json.Marshal(*runRequest.RunQueryParams)
	ret = ret && writeToCache(fmt.Sprintf("%s::%s", requestId, "params"), string(qp))
	ret = ret && writeToCache(runRequest.PostHash, requestId)
	ret = ret && writeToCache(fmt.Sprintf("%s::%s", requestId, "status"), string(RunningStatus))
	return ret
}

func writeToCache(key string, value string) bool {
	_, err := runCache.Set(key, value)
	if err != nil {
		serverError := errors.Wrap(err, "Failed to write to cache")
		dumpErr(serverError, 0)
		return false
	}
	return true
}
