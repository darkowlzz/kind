/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or impliep.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ignite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

type node struct {
	name       string
	binaryPath string
}

func (n *node) String() string {
	return n.name
}

func (n *node) Role() (string, error) {
	cmd := exec.Command(n.binaryPath, "inspect", "vm",
		n.name,
		"--format", fmt.Sprintf(`{{ index .ObjectMeta.Labels "%s"}}`, nodeRoleLabelKey),
	)
	lines, err := exec.OutputLines(cmd)
	if err != nil {
		return "", errors.Wrap(err, "failed to get role for node")
	}
	if len(lines) != 1 {
		return "", errors.Errorf("failed to get role for node: output lines %d != 1", len(lines))
	}

	return lines[0], nil
}

func (n *node) IP() (ipv4 string, ipv6 string, err error) {
	cmd := exec.Command(n.binaryPath, "inspect", "vm", n.name)
	lines, err := exec.CombinedOutputLines(cmd)
	res := strings.Join(lines, "")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get vm details")
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(res), &result)

	status := result["status"].(map[string]interface{})
	ipAddresses := status["ipAddresses"].([]interface{})
	ipAddress := ipAddresses[0].(string)

	return ipAddress, "", nil
}

func (n *node) Command(command string, args ...string) exec.Cmd {
	return &nodeCmd{
		nameOrID:   n.name,
		command:    command,
		args:       args,
		binaryPath: n.binaryPath,
	}
}

// nodeCmd implements exec.Cmd for ignite nodes.
type nodeCmd struct {
	nameOrID   string // The VM name or ID
	command    string
	args       []string
	env        []string
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	binaryPath string
}

func (c *nodeCmd) Run() error {
	args := []string{}

	if c.command == "cp" {
		args = append(args, c.command, c.nameOrID)

		containsSTDIN := false
		stdinIndex := -1
		for i, arg := range c.args {
			if strings.Contains(arg, "/dev/stdin") {
				containsSTDIN = true
				stdinIndex = i
			}
		}
		if containsSTDIN {
			inputBuf := new(bytes.Buffer)
			inputBuf.ReadFrom(c.stdin)

			file, err := ioutil.TempFile("", "kind-file-")
			if err != nil {
				return err
			}
			defer os.Remove(file.Name())

			file.Write(inputBuf.Bytes())

			c.args[stdinIndex] = file.Name()
		}

		args = append(
			args,
			c.args...,
		)
	} else {
		args = append(args, "exec", c.nameOrID)

		// Specify the command and command args.
		cmdWithArgs := []string{c.command}
		cmdWithArgs = append(cmdWithArgs, c.args...)
		fullCmd := fmt.Sprintf("%s", strings.Join(cmdWithArgs, " "))

		args = append(
			args,
			fullCmd,
		)
	}

	cmd := exec.Command(c.binaryPath, args...)
	if c.stdin != nil {
		cmd.SetStdin(c.stdin)
	}
	if c.stderr != nil {
		cmd.SetStderr(c.stderr)
	}
	if c.stdout != nil {
		cmd.SetStdout(c.stdout)
	}

	if err := cmd.Run(); err != nil {
		fmt.Println("node exec failed, retrying...:", err)
		time.Sleep(1 * time.Second)
		return cmd.Run()
	}

	// return cmd.Run()
	return nil
}

func (c *nodeCmd) Start() error {
	args := []string{
		"exec",
	}

	// Specify the VM and command.
	// Specify the VM and command.
	args = append(
		args,
		c.nameOrID,
		c.command,
	)
	args = append(
		args,
		c.args...,
	)
	cmd := exec.Command(c.binaryPath, args...)
	if c.stdin != nil {
		cmd.SetStdin(c.stdin)
	}
	if c.stderr != nil {
		cmd.SetStderr(c.stderr)
	}
	if c.stdout != nil {
		cmd.SetStdout(c.stdout)
	}
	return cmd.Start()
}

func (c *nodeCmd) SetEnv(env ...string) exec.Cmd {
	c.env = env
	return c
}

func (c *nodeCmd) SetStdin(r io.Reader) exec.Cmd {
	c.stdin = r
	return c
}

func (c *nodeCmd) SetStdout(w io.Writer) exec.Cmd {
	c.stdout = w
	return c
}

func (c *nodeCmd) SetStderr(w io.Writer) exec.Cmd {
	c.stderr = w
	return c
}
