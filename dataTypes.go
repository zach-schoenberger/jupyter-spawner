package main

type QueryParams struct {
	UserId          string `form:"uid" binding:"required"`
	UserAddress     string `form:"adr" binding:"required"`
	UserAddressPort int    `form:"prt" binding:"required"`
	Force           bool   `form:"frc"`
}

type PostData struct {
	PythonScript []byte `form:"pyscript" binding:"required"`
}

type RunNotebookRequest struct {
	*QueryParams
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
	RequestId string `json:"requestId"`
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
