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

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

type node struct {
	name string
}

func (n *node) String() string {
	return n.name
}

func (n *node) Role() (string, error) {
	// cmd := exec.Command("ignite", "inspect",
	// 	n.name,
	// )
	// TODO: parse the result and get the role value.
	// lines, err := exec.OutputLines(cmd)
	// if err != nil {
	// 	return "", errors.Wrap(err, "failed to get role for node")
	// }
	// if len(lines) != 1 {
	// 	return "", errors.Errorf()
	// }

	// Return control-plane for now. Single node setup only.
	return "control-plane", nil
}

func (n *node) IP() (ipv4 string, ipv6 string, err error) {
	cmd := exec.Command("ignite", "inspect", "vm", n.name)
	lines, err := exec.CombinedOutputLines(cmd)
	res := strings.Join(lines, "")
	// fmt.Printf("\nINSPECT RESULT: %q", res)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get vm details")
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(res), &result)

	status := result["status"].(map[string]interface{})
	ipAddresses := status["ipAddresses"].([]interface{})
	ipAddress := ipAddresses[0].(string)

	// return "", "", errors.New("failed failed failed")
	// if len(lines) != 1 {
	// 	return "", "", errors.Errorf("file should only be one line, got %d lines", len(lines))
	// }
	// ip := lines[0]
	// fmt.Printf("\nVM IP %q", ip)
	// Return local address for single node setup only.
	return ipAddress, "", nil
}

func (n *node) Command(command string, args ...string) exec.Cmd {
	return &nodeCmd{
		nameOrID: n.name,
		command:  command,
		args:     args,
	}
}

// nodeCmd implements exec.Cmd for ignite nodes.
type nodeCmd struct {
	nameOrID string // The VM name or ID
	command  string
	args     []string
	env      []string
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
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
				fmt.Println("FAILED TO CREATE TEMPORARY FILE:", err)
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

		// fmt.Println("fullCmd:", fullCmd)

		args = append(
			args,
			// c.args...,
			fullCmd,
		)

	}

	// args := []string{
	// 	"exec",
	// }
	// if c.stdin != nil {
	// 	args = append(args,
	// 		"-i",
	// 	)
	// }
	// Set env

	// // Specify the VM.
	// args = append(
	// 	args,
	// 	c.nameOrID,
	// 	c.command,
	// )

	// // Ignite exec doesn't accepts STDIN. Read the input, write to a file and
	// // use that file as source.
	// // fmt.Println("COMMAND ARGS:", c.args)
	// containsSTDIN := false
	// stdinIndex := -1
	// for i, arg := range c.args {
	// 	if strings.Contains(arg, "/dev/stdin") {
	// 		fmt.Println("FOUND STDIN")
	// 		containsSTDIN = true
	// 		stdinIndex = i
	// 	}
	// }
	// if containsSTDIN {
	// 	inputBuf := new(bytes.Buffer)
	// 	inputBuf.ReadFrom(c.stdin)
	// 	// fmt.Println("INPUT:", inputBuf.String())
	// 	cmdcp := exec.Command("ignite", "exec", c.nameOrID, "echo", "-e", inputBuf.String(), ">", "/tmp/cp1")
	// 	if err := cmdcp.Run(); err != nil {
	// 		fmt.Println("FAILED TO CREATE TMP COPY FILE:", err)
	// 		return err
	// 	}
	// 	fmt.Println("WRITTEN TO /tmp/cp1")

	// 	c.args[stdinIndex] = "/tmp/cp1"

	// 	time.Sleep(100)
	// }

	// fmt.Println("FINAL ARGS:", c.args)

	// // Specify the command and command args.
	// cmdWithArgs := []string{c.command}
	// cmdWithArgs = append(cmdWithArgs, c.args...)
	// fullCmd := fmt.Sprintf("%s", strings.Join(cmdWithArgs, " "))

	// fmt.Println("fullCmd:", fullCmd)

	// args = append(
	// 	args,
	// 	c.args...,
	// // fullCmd,
	// )

	cmd := exec.Command("ignite", args...)
	if c.stdin != nil {
		cmd.SetStdin(c.stdin)
	}
	if c.stderr != nil {
		cmd.SetStderr(c.stderr)
	}
	if c.stdout != nil {
		cmd.SetStdout(c.stdout)
	}
	return cmd.Run()
}

func (c *nodeCmd) Start() error {
	args := []string{
		"exec",
	}
	// if c.stdin != nil {
	// 	args = append(args,
	// 		"-i",
	// 	)
	// }
	// Set env

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
	cmd := exec.Command("ignite", args...)
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
