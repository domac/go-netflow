package main

import (
	"bytes"
	"errors"
	"os/exec"
)

//pipe管道命令执行
func Pipeline(cmds ...*exec.Cmd) (pipeLineOutput, collectedStandardError []byte, pipeLineError error) {
	if len(cmds) < 1 {
		return nil, nil, nil
	}

	var output bytes.Buffer
	var stderr bytes.Buffer

	last := len(cmds) - 1
	for i, cmd := range cmds[:last] {
		var err error
		if cmds[i+1].Stdin, err = cmd.StdoutPipe(); err != nil {
			return nil, nil, err
		}
		cmd.Stderr = &stderr
	}

	cmds[last].Stdout, cmds[last].Stderr = &output, &stderr

	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			return output.Bytes(), stderr.Bytes(), err
		}
	}

	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			return output.Bytes(), stderr.Bytes(), err
		}
	}

	return output.Bytes(), stderr.Bytes(), nil
}

//多命令执行
func ExecPipeLine(cmds ...*exec.Cmd) (string, error) {
	output, stderr, err := Pipeline(cmds...)
	if err != nil {
		return "", err
	}

	if len(output) > 0 {
		return string(output), nil
	}

	if len(stderr) > 0 {
		return string(stderr), nil
	}
	return "", errors.New("no returns")
}
