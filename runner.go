package main

import (
	"bytes"
	"encoding/json"
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
