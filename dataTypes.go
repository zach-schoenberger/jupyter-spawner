package main

import (
	"encoding/json"
	"fmt"
)

type Str struct {
	string
}

type RunQueryParams struct {
	UserId          string `form:"uid" binding:"required"`
	UserAddress     string `form:"adr" binding:"required"`
	UserAddressPort int    `form:"prt" binding:"required"`
	Assignment      string `form:"asnmt" binding:"required"`
	Force           bool   `form:"frc"`
}

type PostData struct {
	PythonScript []byte `form:"pyscript" binding:"required"`
}

type RunNotebookRequest struct {
	*RunQueryParams
	*PostData
	RequestId string
	PostHash  string
}

type RunNotebookResponse struct {
	RequestId    string          `json:"requestId"`
	PyscriptHash string          `json:"pyscriptHash"`
	Status       *ResponseStatus `json:"status,omitempty"`
	Error        *string         `json:"error,omitempty"`
	Result       *string         `json:"result,omitempty"`
}

type TemplateData struct {
	JobName      string
	Image        string
	PyScriptHash string
	UserId       string
	RequestId    string
}

type ResultResponse struct {
	Result *string        `json:"result,omitempty"`
	Error  *string        `json:"error,omitempty"`
	Status ResponseStatus `json:"status,omitempty"`
}

type ResultRequest struct {
	RequestId string `uri:"requestId"`
}

type ResponseStatus string

const (
	ErrorStatus    ResponseStatus = "ERROR"
	RunningStatus  ResponseStatus = "RUNNING"
	FinishedStatus ResponseStatus = "FINISHED"
)

func (r ResponseStatus) pointer() *ResponseStatus {
	return &r
}

func (r *RunNotebookResponse) String() string {
	return fmt.Sprintf("{RequestId:%s, PyscriptHash:%s, Status:%s, Result:%s, Error:%s}", r.RequestId, r.PyscriptHash, r.Status, String(r.Result), String(r.Error))
}

func (r *RunNotebookRequest) String() string {
	s, _ := json.Marshal(*r)
	return string(s)
}

func String(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Str) String() string {
	if s == nil {
		return "nil"
	}
	return s.string
}

func (r *ResponseStatus) String() string {
	if r == nil {
		return ""
	}
	return string(*r)
}
