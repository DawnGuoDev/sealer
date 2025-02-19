// Copyright © 2022 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k0s

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/sealerio/sealer/common"
	"github.com/sealerio/sealer/pkg/registry"
	"github.com/sealerio/sealer/pkg/runtime"
	v2 "github.com/sealerio/sealer/types/api/v2"
	"github.com/sealerio/sealer/utils/platform"
	"github.com/sealerio/sealer/utils/ssh"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Runtime struct is the runtime interface for k0s
type Runtime struct {
	// cluster is sealer clusterFile
	cluster   *v2.Cluster
	Vlog      int
	RegConfig *registry.Config
}

func (k *Runtime) Init() error {
	return k.init()
}

func (k *Runtime) Upgrade() error {
	return k.upgrade()
}

func (k *Runtime) Reset() error {
	logrus.Infof("Start to delete cluster: master %s, node %s", k.cluster.GetMasterIPList(), k.cluster.GetNodeIPList())
	return k.reset()
}

func (k *Runtime) JoinMasters(newMastersIPList []net.IP) error {
	if len(newMastersIPList) != 0 {
		logrus.Infof("%s will be added as master", newMastersIPList)
	}
	return k.joinMasters(newMastersIPList)
}

func (k *Runtime) JoinNodes(newNodesIPList []net.IP) error {
	if len(newNodesIPList) != 0 {
		logrus.Infof("%s will be added as worker", newNodesIPList)
	}
	return k.joinNodes(newNodesIPList)
}

func (k *Runtime) DeleteMasters(mastersIPList []net.IP) error {
	if len(mastersIPList) != 0 {
		logrus.Infof("master %s will be deleted", mastersIPList)
		return k.deleteMasters(mastersIPList)
	}
	return nil
}

func (k *Runtime) DeleteNodes(nodesIPList []net.IP) error {
	if len(nodesIPList) != 0 {
		logrus.Infof("worker %s will be deleted", nodesIPList)
		return k.deleteNodes(nodesIPList)
	}
	return nil
}

func (k *Runtime) GetClusterMetadata() (*runtime.Metadata, error) {
	return k.getClusterMetadata()
}

// NewK0sRuntime arg "clusterConfig" is the k0s config file under etc/${ant_name.yaml}, runtime need read k0s config from it
// Mount image is required before new Runtime.
func NewK0sRuntime(cluster *v2.Cluster) (runtime.Installer, error) {
	return newK0sRuntime(cluster)
}

func newK0sRuntime(cluster *v2.Cluster) (runtime.Installer, error) {
	k := &Runtime{
		cluster: cluster,
	}

	k.RegConfig = registry.GetConfig(k.getImageMountDir(), k.cluster.GetMaster0IP())

	if err := k.checkList(); err != nil {
		return nil, err
	}

	setDebugLevel(k)
	// todo need to adapt the new runtime interface.temporarily return nil
	return nil, nil
	//return k, nil
}

func setDebugLevel(k *Runtime) {
	if logrus.GetLevel() == logrus.DebugLevel {
		k.Vlog = 6
	}
}

// checkList do a simple check for cluster spec Hosts which can not be empty.
func (k *Runtime) checkList() error {
	if len(k.cluster.Spec.Hosts) == 0 {
		return fmt.Errorf("hosts spec cannot be empty")
	}
	if k.cluster.GetMaster0IP() == nil {
		return fmt.Errorf("master0 IP cannot be empty")
	}
	return nil
}

// getImageMountDir return a path for mount dir, eg: /var/lib/sealer/data/my-k0s-cluster/mount
func (k *Runtime) getImageMountDir() string {
	return platform.DefaultMountClusterImageDir(k.cluster.Name)
}

// getHostSSHClient return ssh client with destination machine.
func (k *Runtime) getHostSSHClient(hostIP net.IP) (ssh.Interface, error) {
	return ssh.NewStdoutSSHClient(hostIP, k.cluster)
}

// getRootfs return the rootfs dir like: /var/lib/sealer/data/my-k0s-cluster/rootfs
func (k *Runtime) getRootfs() string {
	return common.DefaultTheClusterRootfsDir(k.cluster.Name)
}

// getCertsDir return a Dir value such as: /var/lib/sealer/data/my-k0s-cluster/certs
func (k *Runtime) getCertsDir() string {
	return common.TheDefaultClusterCertDir(k.cluster.Name)
}

// sendFileToHosts sent file to the dst dir on host machine
func (k *Runtime) sendFileToHosts(Hosts []net.IP, src, dst string) error {
	eg, _ := errgroup.WithContext(context.Background())
	for _, node := range Hosts {
		node := node
		eg.Go(func() error {
			sshClient, err := k.getHostSSHClient(node)
			if err != nil {
				return fmt.Errorf("failed to send file: %v", err)
			}
			if err := sshClient.Copy(node, src, dst); err != nil {
				return fmt.Errorf("failed to send file: %v", err)
			}
			return err
		})
	}
	return eg.Wait()
}

func (k *Runtime) WaitSSHReady(tryTimes int, hosts ...net.IP) error {
	eg, _ := errgroup.WithContext(context.Background())
	for _, h := range hosts {
		host := h
		eg.Go(func() error {
			for i := 0; i < tryTimes; i++ {
				sshClient, err := k.getHostSSHClient(host)
				if err != nil {
					return err
				}
				err = sshClient.Ping(host)
				if err == nil {
					return nil
				}
				time.Sleep(time.Duration(i) * time.Second)
			}
			return fmt.Errorf("wait for [%s] ssh ready timeout, ensure that the IP address or password is correct", host)
		})
	}
	return eg.Wait()
}

func (k *Runtime) CopyJoinToken(role string, hosts []net.IP) error {
	var joinCertPath string
	ssh, err := k.getHostSSHClient(k.cluster.GetMaster0IP())
	if err != nil {
		return err
	}
	switch role {
	case ControllerRole:
		joinCertPath = DefaultK0sControllerJoin
	case WorkerRole:
		joinCertPath = DefaultK0sWorkerJoin
	default:
		joinCertPath = DefaultK0sWorkerJoin
	}

	eg, _ := errgroup.WithContext(context.Background())
	for _, host := range hosts {
		host := host
		eg.Go(func() error {
			return ssh.Copy(host, joinCertPath, joinCertPath)
		})
	}
	return nil
}

func (k *Runtime) JoinCommand(role string) []string {
	cmds := map[string][]string{
		ControllerRole: {"mkdir -r /etc/k0s", fmt.Sprintf("k0s config create > %s", DefaultK0sConfigPath),
			fmt.Sprintf("sed -i '/  images/ a\\    repository: \"%s:%s\"' %s", k.RegConfig.Domain, k.RegConfig.Port, DefaultK0sConfigPath),
			fmt.Sprintf("k0s install controller --token-file %s -c %s --cri-socket %s",
				DefaultK0sControllerJoin, DefaultK0sConfigPath, ExternalCRI),
			"k0s start",
		},
		WorkerRole: {fmt.Sprintf("k0s install worker --cri-socket %s --token-file %s", ExternalCRI, DefaultK0sWorkerJoin),
			"k0s start"},
	}

	v, ok := cmds[role]
	if !ok {
		return nil
	}
	return v
}

// CmdToString is in host exec cmd and replace to spilt str
func (k *Runtime) CmdToString(host net.IP, cmd, split string) (string, error) {
	ssh, err := k.getHostSSHClient(host)
	if err != nil {
		return "", fmt.Errorf("failed to get ssh clientof host(%s): %v", host, err)
	}
	return ssh.CmdToString(host, cmd, split)
}

func (k *Runtime) getClusterMetadata() (*runtime.Metadata, error) {
	metadata := &runtime.Metadata{}

	version, err := k.getKubeVersion()
	if err != nil {
		return metadata, err
	}
	metadata.Version = version
	metadata.ClusterRuntime = RuntimeFlag
	return metadata, nil
}

func (k *Runtime) getKubeVersion() (string, error) {
	ssh, err := k.getHostSSHClient(k.cluster.GetMaster0IP())
	if err != nil {
		return "", err
	}
	bytes, err := ssh.Cmd(k.cluster.GetMaster0IP(), VersionCmd)
	if err != nil {
		return "", err
	}
	return strings.Split(string(bytes), "+")[0], nil
}
