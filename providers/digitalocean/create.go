/*
Copyright (C) 2021-2023, Kubefirst

This program is licensed under MIT.
See the LICENSE file for more details.
*/
package digitalocean

import (
	"os"
	"time"

	"github.com/kubefirst/kubefirst-api/internal/controller"
	"github.com/kubefirst/kubefirst-api/internal/telemetryShim"
	"github.com/kubefirst/kubefirst-api/internal/types"
	"github.com/kubefirst/runtime/pkg/digitalocean"
	"github.com/kubefirst/runtime/pkg/k8s"
	"github.com/kubefirst/runtime/pkg/segment"
	"github.com/kubefirst/runtime/pkg/ssl"
	log "github.com/sirupsen/logrus"
)

// CreateDigitaloceanCluster
func CreateDigitaloceanCluster(definition *types.ClusterDefinition) error {
	ctrl := controller.ClusterController{}
	err := ctrl.InitController(definition)
	if err != nil {
		return err
	}

	err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", true)
	if err != nil {
		return err
	}

	err = ctrl.DownloadTools(ctrl.ProviderConfig.(*digitalocean.DigitaloceanConfig).ToolsDir)
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.DomainLivenessTest()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.StateStoreCredentials()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.GitInit()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.InitializeBot()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.RepositoryPrep()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.RunGitTerraform()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.RepositoryPush()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.CreateCluster()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	// Needs wait after cluster create
	time.Sleep(time.Second * 30)

	err = ctrl.ClusterSecretsBootstrap()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	//* check for ssl restore
	log.Info("checking for tls secrets to restore")
	secretsFilesToRestore, err := os.ReadDir(ctrl.ProviderConfig.(*digitalocean.DigitaloceanConfig).SSLBackupDir + "/secrets")
	if err != nil {
		log.Infof("%s", err)
	}
	if len(secretsFilesToRestore) != 0 {
		// todo would like these but requires CRD's and is not currently supported
		// add crds ( use execShellReturnErrors? )
		// https://raw.githubusercontent.com/cert-manager/cert-manager/v1.11.0/deploy/crds/crd-clusterissuers.yaml
		// https://raw.githubusercontent.com/cert-manager/cert-manager/v1.11.0/deploy/crds/crd-certificates.yaml
		// add certificates, and clusterissuers
		log.Infof("found %d tls secrets to restore", len(secretsFilesToRestore))
		ssl.Restore(ctrl.ProviderConfig.(*digitalocean.DigitaloceanConfig).SSLBackupDir, ctrl.DomainName, ctrl.ProviderConfig.(*digitalocean.DigitaloceanConfig).Kubeconfig)
	} else {
		log.Info("no files found in secrets directory, continuing")
	}

	err = ctrl.InstallArgoCD()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.InitializeArgoCD()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.DeployRegistryApplication()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.WaitForVault()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.InitializeVault()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	//
	kcfg := k8s.CreateKubeConfig(false, ctrl.ProviderConfig.(*digitalocean.DigitaloceanConfig).Kubeconfig)

	// SetupMinioStorage(kcfg, ctrl.ProviderConfig.K1Dir, ctrl.GitProvider)

	//* configure vault with terraform
	//* vault port-forward
	vaultStopChannel := make(chan struct{}, 1)
	defer func() {
		close(vaultStopChannel)
	}()
	k8s.OpenPortForwardPodWrapper(
		kcfg.Clientset,
		kcfg.RestConfig,
		"vault-0",
		"vault",
		8200,
		8200,
		vaultStopChannel,
	)

	err = ctrl.RunVaultTerraform()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.RunUsersTerraform()
	if err != nil {
		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	// Wait for console Deployment Pods to transition to Running
	consoleDeployment, err := k8s.ReturnDeploymentObject(
		kcfg.Clientset,
		"app.kubernetes.io/instance",
		"kubefirst-console",
		"kubefirst",
		600,
	)
	if err != nil {
		log.Errorf("Error finding console Deployment: %s", err)

		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}
	_, err = k8s.WaitForDeploymentReady(kcfg.Clientset, consoleDeployment, 120)
	if err != nil {
		log.Errorf("Error waiting for console Deployment ready state: %s", err)

		err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
		if err != nil {
			return err
		}

		return err
	}

	err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "status", "provisioned")
	if err != nil {
		return err
	}

	err = ctrl.MdbCl.UpdateCluster(ctrl.ClusterName, "in_progress", false)
	if err != nil {
		return err
	}

	log.Info("cluster creation complete")

	// Telemetry handler
	rec, err := ctrl.GetCurrentClusterRecord()
	if err != nil {
		return err
	}

	segmentClient, err := telemetryShim.SetupTelemetry(rec)
	if err != nil {
		return err
	}
	defer segmentClient.Client.Close()

	telemetryShim.Transmit(rec.UseTelemetry, segmentClient, segment.MetricMgmtClusterInstallCompleted, "")

	return nil
}