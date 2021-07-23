package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	cnitypes "github.com/containernetworking/cni/pkg/types"
	cnicurrent "github.com/containernetworking/cni/pkg/types/current"
	"github.com/cri-o/cri-o/internal/lib/sandbox"
	"github.com/cri-o/cri-o/internal/log"
	"github.com/cri-o/cri-o/server/metrics"
	"github.com/pkg/errors"
)

// networkStart sets up the sandbox's network and returns the pod IP on success
// or an error
func (s *Server) networkStart(ctx context.Context, sb *sandbox.Sandbox) (podIPs []string, result cnitypes.Result, retErr error) {
	overallStart := time.Now()
	// give a network Start call 2 minutes, half of a RunPodSandbox request timeout limit
	startCtx, startCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer startCancel()

	if sb.HostNetwork() {
		return nil, nil, nil
	}

	podNetwork, err := s.newPodNetwork(sb)
	if err != nil {
		return nil, nil, err
	}

	// Ensure network resources are cleaned up if the plugin succeeded
	// but an error happened between plugin success and the end of networkStart()
	defer func() {
		if retErr != nil {
			log.Infof(ctx, "networkStart: stopping network for sandbox %s", sb.ID())
			if err2 := s.networkStop(ctx, sb); err2 != nil {
				log.Errorf(ctx, "error stopping network on cleanup: %v", err2)
			}
		}
	}()

	podSetUpStart := time.Now()
	_, err = s.config.CNIPlugin().SetUpPodWithContext(startCtx, podNetwork)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pod network sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
	}
	// metric about the CNI network setup operation
	metrics.CRIOOperationsLatency.WithLabelValues("network_setup_pod").
		Observe(metrics.SinceInMicroseconds(podSetUpStart))

	podNetworkStatus, err := s.config.CNIPlugin().GetPodNetworkStatusWithContext(startCtx, podNetwork)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network status for pod sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
	}

	// only one cnitypes.Result is returned since newPodNetwork sets Networks list empty
	result = podNetworkStatus[0].Result
	log.Debugf(ctx, "CNI setup result: %v", result)

	network, err := cnicurrent.GetResult(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network JSON for pod sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
	}

	for _, podIPConfig := range network.IPs {
		podIP := strings.Split(podIPConfig.Address.String(), "/")[0]
		log.Infof(ctx, "Skipped use of hostport manager add...")
		podIPs = append(podIPs, podIP)
	}

	log.Debugf(ctx, "found POD IPs: %v", podIPs)

	// metric about the whole network setup operation
	metrics.CRIOOperationsLatency.WithLabelValues("network_setup_overall").
		Observe(metrics.SinceInMicroseconds(overallStart))
	return podIPs, result, err
}

// getSandboxIP retrieves the IP address for the sandbox
func (s *Server) getSandboxIPs(sb *sandbox.Sandbox) (podIPs []string, err error) {
	if sb.HostNetwork() {
		return nil, nil
	}

	podNetwork, err := s.newPodNetwork(sb)
	if err != nil {
		return nil, err
	}
	podNetworkStatus, err := s.config.CNIPlugin().GetPodNetworkStatus(podNetwork)
	if err != nil {
		return nil, fmt.Errorf("failed to get network status for pod sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
	}

	res, err := cnicurrent.GetResult(podNetworkStatus[0].Result)
	if err != nil {
		return nil, fmt.Errorf("failed to get network JSON for pod sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
	}

	for _, podIPConfig := range res.IPs {
		podIPs = append(podIPs, strings.Split(podIPConfig.Address.String(), "/")[0])
	}

	return podIPs, nil
}

// networkStop cleans up and removes a pod's network.  It is best-effort and
// must call the network plugin even if the network namespace is already gone
func (s *Server) networkStop(ctx context.Context, sb *sandbox.Sandbox) error {
	if sb.HostNetwork() || sb.NetworkStopped() {
		return nil
	}
	// give a network stop call 1 minutes, half of a StopPod request timeout limit
	stopCtx, stopCancel := context.WithTimeout(ctx, 1*time.Minute)
	defer stopCancel()

	log.Infof(ctx, "Skipped use of hostport manager remove...")

	podNetwork, err := s.newPodNetwork(sb)
	if err != nil {
		return err
	}
	if err := s.config.CNIPlugin().TearDownPodWithContext(stopCtx, podNetwork); err != nil {
		return errors.Wrapf(err, "failed to destroy network for pod sandbox %s(%s)", sb.Name(), sb.ID())
	}

	return sb.SetNetworkStopped(true)
}
