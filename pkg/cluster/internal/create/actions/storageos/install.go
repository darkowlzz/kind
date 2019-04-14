package storageos

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"sigs.k8s.io/kind/pkg/cluster/constants"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

type Action struct{}

func NewAction() actions.Action {
	return &Action{}
}

func (a *Action) Execute(ctx *actions.ActionContext) error {
	ctx.Status.Start("Starting storageos 💾")
	defer ctx.Status.End(false)

	allNodes, err := ctx.Nodes()
	if err != nil {
		return err
	}

	var cpNode nodes.Node
	for _, node := range allNodes {
		role, err := node.Role()
		if err != nil {
			return err
		}

		if role == constants.ControlPlaneNodeRoleValue {
			cpNode = node
		}
	}

	fmt.Println("Setting up storageos from node", cpNode.Name())

	cmd := cpNode.Command(
		"kubectl", "apply", "-f",
		"https://gist.githubusercontent.com/darkowlzz/a32f1474151abd9a7f9a79ce563004c2/raw/ae42181afd4a28ac64ad4aa6df22fb27e51fdf9e/storageos-operator-deploy.yaml",
		"--kubeconfig=/etc/kubernetes/admin.conf",
	)
	lines, err := exec.CombinedOutputLines(cmd)
	log.Debug(strings.Join(lines, "\n"))
	if err != nil {
		return errors.Wrap(err, "failed to setup storageos")
	}

	fmt.Printf("\nWaiting ≤ %s for StorageOS Operator = Ready ⏳\n", "500s")
	startTime := time.Now()
	operatorIsReady := WaitForReadyOperator(&cpNode, startTime.Add(500*time.Second))
	if !operatorIsReady {
		ctx.Status.End(false)
		fmt.Println(" • WARNING: Timed out waiting for StorageOS Operator to be Ready ⚠")
		return nil
	}
	fmt.Println("StorageOS Operator - Ready!")

	fmt.Printf("\nWaiting ≤ %s for StorageOS = Ready ⏳\n", "500s")
	startTime = time.Now()
	storageosIsReady := WaitForReadyStorageOS(&cpNode, startTime.Add(500*time.Second))
	if !storageosIsReady {
		ctx.Status.End(false)
		fmt.Println(" • WARNING: Timed out waiting for StorageOS to be Ready ⚠")
		return nil
	}
	fmt.Println("StorageOS - Ready!")

	ctx.Status.End(true)
	return nil
}

// WaitForReadyOperator uses kubectl inside the "node" container to check if the
// StorageOS operator are "Ready".
func WaitForReadyOperator(node *nodes.Node, until time.Time) bool {
	return tryUntil(until, func() bool {
		cmd := node.Command(
			"kubectl",
			"--kubeconfig=/etc/kubernetes/admin.conf",
			"-n", "storageos-operator",
			"get",
			"pods",
			// When the pod reaches status ready, the status field will be set
			// to true.
			"-o=jsonpath='{.items..status.conditions[-1:].status}'",
		)
		lines, err := exec.CombinedOutputLines(cmd)
		if err != nil {
			return false
		}

		// 'lines' will return the status of storageos-operator pods.
		status := strings.Fields(lines[0])
		for _, s := range status {
			// Check pod status. If node is ready then this wil be 'True',
			// 'False' or 'Unkown' otherwise.
			if !strings.Contains(s, "True") {
				return false
			}
		}
		return true
	})
}

// WaitForReadyStorageOS uses kubectl inside the "node" container to check if the
// StorageOS pods are "Running".
func WaitForReadyStorageOS(node *nodes.Node, until time.Time) bool {
	return tryUntil(until, func() bool {
		cmd := node.Command(
			"kubectl",
			"--kubeconfig=/etc/kubernetes/admin.conf",
			"-n", "storageos",
			"get",
			"pods",
			// When the pod reaches status ready, the status field will be set
			// to true.
			"-o=jsonpath='{.items..status.phase}'",
		)
		lines, err := exec.CombinedOutputLines(cmd)
		if err != nil {
			return false
		}

		// 'lines' will return the status of all storageos pods. For
		// example, if we have three storageos pods, and all are ready,
		// then the status will have the following format:
		// `Running Running Running'.
		status := strings.Fields(lines[0])
		for _, s := range status {
			// Check pod status. If node is ready then this will be 'Running'.
			if !strings.Contains(s, "Running") {
				return false
			}
		}
		return true
	})
}

// helper that calls `try()`` in a loop until the deadline `until`
// has passed or `try()`returns true, returns wether try ever returned true
func tryUntil(until time.Time, try func() bool) bool {
	for until.After(time.Now()) {
		if try() {
			return true
		}
	}
	return false
}
