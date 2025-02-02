// Copyright 2021 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/chaos-mesh/chaos-mesh/controllers/config"
	chaosdaemonclient "github.com/chaos-mesh/chaos-mesh/pkg/chaosdaemon/client"
	grpcUtils "github.com/chaos-mesh/chaos-mesh/pkg/grpc"
	"github.com/chaos-mesh/chaos-mesh/pkg/mock"
)

var log = ctrl.Log.WithName("controller-chaos-daemon-client-utils")

func FindDaemonIP(ctx context.Context, c client.Client, pod *v1.Pod) (string, error) {
	nodeName := pod.Spec.NodeName
	log.Info("Creating client to chaos-daemon", "node", nodeName)

	ns := config.ControllerCfg.Namespace
	var endpoints v1.Endpoints
	err := c.Get(ctx, types.NamespacedName{
		Namespace: ns,
		Name:      "chaos-daemon",
	}, &endpoints)
	if err != nil {
		return "", err
	}

	daemonIP := findIPOnEndpoints(&endpoints, nodeName)
	if len(daemonIP) == 0 {
		return "", errors.Errorf("cannot find daemonIP on node %s in related Endpoints %v", nodeName, endpoints)
	}

	return daemonIP, nil
}

func findIPOnEndpoints(e *v1.Endpoints, nodeName string) string {
	for _, subset := range e.Subsets {
		for _, addr := range subset.Addresses {
			if addr.NodeName != nil && *addr.NodeName == nodeName {
				return addr.IP
			}
		}
	}

	return ""
}

// NewChaosDaemonClient would create ChaosDaemonClient
func NewChaosDaemonClient(ctx context.Context, c client.Client, pod *v1.Pod) (chaosdaemonclient.ChaosDaemonClientInterface, error) {
	if cli := mock.On("MockChaosDaemonClient"); cli != nil {
		return cli.(chaosdaemonclient.ChaosDaemonClientInterface), nil
	}
	if err := mock.On("NewChaosDaemonClientError"); err != nil {
		return nil, err.(error)
	}

	daemonIP, err := FindDaemonIP(ctx, c, pod)
	if err != nil {
		return nil, err
	}
	builder := grpcUtils.Builder(daemonIP, config.ControllerCfg.ChaosDaemonPort).WithDefaultTimeout()
	if config.ControllerCfg.TLSConfig.ChaosMeshCACert != "" {
		builder.TLSFromFile(config.ControllerCfg.TLSConfig.ChaosMeshCACert, config.ControllerCfg.TLSConfig.ChaosDaemonClientCert, config.ControllerCfg.TLSConfig.ChaosDaemonClientKey)
	} else {
		builder.Insecure()
	}
	cc, err := builder.Build()
	if err != nil {
		return nil, err
	}
	return chaosdaemonclient.New(cc), nil
}
