package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"net/http"
	"net/http/httputil"
)

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

	writeToCache(fmt.Sprintf("%s::%s", resultRequest.RequestId, "status"), string(FinishedStatus))
	writeToCache(fmt.Sprintf("%s::%s", resultRequest.RequestId, "result"), string(postData.PythonScript))
	p, err := runCache.Get(fmt.Sprintf("%s::%s", resultRequest.RequestId, "params"))
	if err == nil {
		params := RunQueryParams{}
		err := json.Unmarshal([]byte(p), &params)
		if err == nil {
			uri := fmt.Sprintf("http://%s:%d", params.UserAddress, params.UserAddressPort)
			log.Infof("Sending to %s", uri)
			data := getResultResponse(resultRequest.RequestId, postData.PythonScript)
			resp, err := http.Post(uri, "application/json", bytes.NewReader(data))
			if err == nil {
				if respStr, err := httputil.DumpResponse(resp, true); err != nil {
					log.Infof("%s", respStr)
				}
			}
		}
	}

	return http.StatusOK, ResultResponse{
		Status: FinishedStatus,
	}
}

func getResultResponse(requestId string, result []byte) []byte {
	r := string(result)
	resp := RunNotebookResponse{
		RequestId:    requestId,
		PyscriptHash: "",
		Status:       FinishedStatus.pointer(),
		Result:       &r,
	}
	data, _ := json.Marshal(resp)
	return data
}

func getRunResults(runRequest *RunNotebookRequest) (requestId string, status ResponseStatus, data string, err error) {
	if requestId, err = runCache.Get(runRequest.PostHash); err != nil {
		return "", "", "", errors.WithStack(err)
	}
	statusStr, err := runCache.Get(fmt.Sprintf("%s::%s", requestId, "status"))
	if err != nil {
		return "", "", "", errors.WithStack(err)
	}
	status = ResponseStatus(statusStr)
	if statusStr != string(FinishedStatus) {
		return requestId, status, "", nil
	}
	if data, err = runCache.Get(fmt.Sprintf("%s::%s", requestId, "result")); err != nil {
		return "", "", "", errors.WithStack(err)
	}

	return requestId, status, data, nil
}
