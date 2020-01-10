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
	"os/exec"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/internal/providers/provider/common"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/log"
)

func ensureNodeImages(logger log.Logger, status *cli.Status, cfg *config.Cluster) {
	// Pull each required image.
	for _, image := range common.RequiredNodeImages(cfg).List() {
		// Prints user friendly message.
		friendlyImageName := image
		if strings.Contains(image, "@sha256:") {
			friendlyImageName = strings.Split(image, "@sha256:")[0]
		}
		status.Start(fmt.Sprintf("Ensuring node image (%s) 🖼", friendlyImageName))

		// Attempt to explicitly pull the image if it doesn't exist locally.
		// We don't care if this errors, we'll still try to run which also
		// pulls.
		_, _ = pullIfNotPresent(logger, image, 4)
	}
}

func pullIfNotPresent(logger log.Logger, image string, retries int) (pulled bool, err error) {
	// Ignite doesn't provide any way to filter or inspect images. Pull the
	// image always for now.
	// cmd := exec.Command("ignite", "image", "import")

	return true, pull(logger, image, retries)
}

func pull(logger log.Logger, image string, retries int) error {
	logger.V(1).Infof("Pulling image: %s ...", image)
	err := exec.Command("ignite", "image", "import").Run()
	if err != nil {
		for i := 0; i < retries; i++ {
			time.Sleep(time.Second * time.Duration(i+1))
			logger.V(1).Infof("Trying again to pull image: %q ... %v", image, err)
			err = exec.Command("ignite", "image", "import", image).Run()
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		logger.V(1).Infof("Failed to pull image: %q %v", image, err)
	}
	return err
}
