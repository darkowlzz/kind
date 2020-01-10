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
	"sigs.k8s.io/kind/pkg/cluster/internal/providers/provider/common"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// planCreation creates a slice of funcs that will create the VMs.
func planCreation(cluster string, cfg *config.Cluster) (createVMFuncs []func() error, err error) {
	// These apply to all VM creation.
	nodeNamer := common.MakeNodeNamer(cluster)
	genericArgs, err := commonArgs(cluster, cfg)
	if err != nil {
		return nil, err
	}

	apiServerPort := cfg.Networking.APIServerPort
	apiServerAddress := cfg.Networking.APIServerAddress

	// TODO: Check for loadbalancer in config.

	// Plan normal nodes.
	for _, node := range cfg.Nodes {
		node := node.DeepCopy()              // Copy so we can modify.
		name := nodeNamer(string(node.Role)) // Name the node.

		// Plan actual creation based on role.
		switch node.Role {
		case config.ControlPlaneRole:
			createVMFuncs = append(createVMFuncs, func() error {
				node.ExtraPortMappings = append(node.ExtraPortMappings,
					config.PortMapping{
						ListenAddress: apiServerAddress,
						HostPort:      apiServerPort,
						// ContainerPort: common.APIServerInternalPort,
					},
				)
				return createVM(runArgsForNode(node, name, genericArgs))
			})
		case config.WorkerRole:
			createVMFuncs = append(createVMFuncs, func() error {
				return createVM(runArgsForNode(node, name, genericArgs))
			})
		default:
			return nil, errors.Errorf("unknown node role: %q", node.Role)
		}
	}
	return createVMFuncs, nil
}

func createVM(args []string) error {
	if err := exec.Command("ignite", args...).Run(); err != nil {
		return errors.Wrap(err, "ignite run error")
	}
	return nil
}

func commonArgs(cluster string, cfg *config.Cluster) ([]string, error) {
	args := []string{}

	return args, nil
}

func runArgsForNode(node *config.Node, name string, args []string) []string {
	args = append([]string{
		"run",
		"--name", name,
		"--cpus", "1",
		"--memory", "2GB",
		"--ssh",
		// Add label when ignite supports it.
	},
		args...,
	)

	// Finally, specify the image to run.
	return append(args, node.Image)
}
