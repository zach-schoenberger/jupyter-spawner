package main

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"os/exec"
)

func saveFile(filename string, fileData []byte) error {
	return ioutil.WriteFile(getFullFileName(filename), fileData, 0644)
}

func deleteFile(filename string) error {
	return os.Remove(filename)
}

func convertScript(pyscript string) ([]byte, error) {
	cmd := exec.Command("jupyter", "nbconvert", "--to", "script", getFullFileName(pyscript))
	return runCommand(cmd)
}

func compile(filename string) ([]byte, error) {
	cmd := exec.Command("python", "-m", "compileall", getFullFileName(filename))
	return runCommand(cmd)
}

func readFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

func runCommand(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

func getFullFileName(fileName string) string {
	return fmt.Sprintf("/tmp/%s", fileName)
}

func processScript(requestId string, data []byte) ([]byte, error) {
	var err error
	var rb []byte

	if err = saveFile(requestId+".ipynb", data); err != nil {
		return nil, errors.WithStack(err)
	}
	if rb, err = convertScript(requestId + ".ipynb"); err != nil {
		return nil, errors.WithStack(err)
	}
	if rb, err = compile(requestId + ".py"); err != nil {
		return nil, errors.WithStack(err)
	}
	_ = deleteFile(requestId + ".ipynb")
	_ = deleteFile(requestId + ".py")
	_ = deleteFile(requestId + ".pyc")
	return rb, nil
}
