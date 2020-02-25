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
	"fmt"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/kind/pkg/cluster/internal/providers/provider"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/log"
)

// NewProvider returns a new provider based on executing `ignite ...`
func NewProvider(binaryPath string, logger log.Logger) provider.Provider {
	return &Provider{
		logger:     logger,
		BinaryPath: binaryPath,
	}
}

// Provider implements provider.Provider
// see NewProvider
type Provider struct {
	logger     log.Logger
	BinaryPath string
}

// Provision is part of the providers.Provider interface. It provisions a
// cluster using ignite.
func (p *Provider) Provision(status *cli.Status, cluster string, cfg *config.Cluster) (err error) {
	// TODO: validate cfg
	// Ensure node images are pulled before actual provisioning.
	ensureNodeImages(p.BinaryPath, p.logger, status, cfg)

	// Actually provision the cluster.
	icons := strings.Repeat("📦 ", len(cfg.Nodes))
	status.Start(fmt.Sprintf("Preparing nodes %s", icons))
	defer func() { status.End(err == nil) }()

	// Plan creating the VMs.
	createVMFuncs, err := planCreation(p.BinaryPath, cluster, cfg)
	if err != nil {
		return err
	}

	// Actually create nodes.
	// return errors.UntilErrorConcurrent()
	// return errors.UntilErrorConcurrent(createVMFuncs)
	return errors.UntilErrorSync(createVMFuncs)
}

// ListClusters is part of the providers.Provider interface. It lists all the
// ignite kind clusters.
func (p *Provider) ListClusters() ([]string, error) {
	cmd := exec.Command(p.BinaryPath,
		"ps",
		"-q",
		"-a",
		// Filter for nodes with cluster label.
		"--filter", "{{.ObjectMeta.Labels}}=~"+clusterLabelKey,
		// Format to include the cluster name.
		"--format", fmt.Sprintf(`{{index .ObjectMeta.Labels "%s"}}`, clusterLabelKey),
	)
	lines, err := exec.OutputLines(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list clusters")
	}
	return sets.NewString(lines...).List(), nil
}

// ListNodes is part of the providers.Provider interface. It lists all the nodes
// of a given ignite kind cluster.
func (p *Provider) ListNodes(cluster string) ([]nodes.Node, error) {
	cmd := exec.Command(p.BinaryPath,
		"ps",
		"-q",
		"-a",
		// Filter for nodes with cluster label.
		"--filter", fmt.Sprintf(`{{.ObjectMeta.Labels}}=~%s:%s`, clusterLabelKey, cluster),
		// Format to include the cluster name.
		"--format", `{{.ObjectMeta.Name}}`,
	)
	lines, err := exec.OutputLines(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list cluster nodes")
	}

	// Convert names to node handles.
	ret := make([]nodes.Node, 0, len(lines))
	for _, name := range lines {
		name = strings.TrimSpace(name)
		ret = append(ret, p.node(name))
	}
	return ret, nil
	// return []nodes.Node{}, nil
}

// DeleteNodes is part of the providers.Provider interface. It deletes the given
// ignite nodes.
func (p *Provider) DeleteNodes(n []nodes.Node) error {
	if len(n) == 0 {
		return nil
	}
	args := make([]string, 0, len(n)+3) // Allocate once.
	args = append(args,
		"rm",
		"-f",
	)
	for _, node := range n {
		args = append(args, node.String())
	}
	if err := exec.Command(p.BinaryPath, args...).Run(); err != nil {
		return errors.Wrap(err, "failed to delete nodes")
	}
	return nil
}

// GetAPIServerEndpoint is part of the providers.Provider interface. It returns
// the API Server Endpoint.
func (p *Provider) GetAPIServerEndpoint(cluster string) (string, error) {
	// Locate the node that hosts this.
	allNodes, err := p.ListNodes(cluster)
	if err != nil {
		return "", errors.Wrap(err, "failed to list nodes")
	}
	n, err := nodeutils.APIServerEndpointNode(allNodes)
	if err != nil {
		return "", errors.Wrap(err, "failed to get api server endpoint")
	}

	ipv4, _, err := n.IP()
	if err != nil {
		return "", errors.Wrap(err, "failed to get node IP")
	}

	return net.JoinHostPort(ipv4, "6443"), nil
}

func (p *Provider) node(name string) nodes.Node {
	return &node{
		name:       name,
		binaryPath: p.BinaryPath,
	}
}
