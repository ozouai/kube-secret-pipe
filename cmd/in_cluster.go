package main

import (
	"os"
	"sync"

	"github.com/ozouai/kubesecretpipe"
	"github.com/spf13/cobra"
	"gopkg.zouai.io/colossus/clog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs KubeSecretPipe with Kubernetes secrets provided by the pod",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		config, err := kubesecretpipe.ParseConfigFile(rootCtx, *flagConfigFilePath)
		if err != nil {
			clog.Errf(rootCtx, err, "Error Parsing Config File")
			os.Exit(1)
		}
		var kubernetesClient *rest.Config
		if *flagInClusterCreds {
			kubernetesClient, err = rest.InClusterConfig()
			if err != nil {
				clog.Errf(rootCtx, err, "Error Reading Kubernetes Cluster Credentials")
				os.Exit(1)
			}
		}
		if *flagKubeConfigFile != "" {
			if kubernetesClient != nil {
				clog.Errorf(rootCtx, "Only one of `--in-cluster-creds, --kubeconfig-creds` can be specified at a time")
				os.Exit(1)
			}
			kubernetesClient, err = clientcmd.BuildConfigFromFlags("", *flagKubeConfigFile)
			if err != nil {
				clog.Errf(rootCtx, err, "Error Reading Kubernetes Config File")
				os.Exit(1)
			}
		}
		if kubernetesClient == nil {
			clog.Errorf(rootCtx, "You must specify one credentials flag.")
			os.Exit(1)
		}
		clientset, err := kubernetes.NewForConfig(kubernetesClient)
		if err != nil {
			clog.Errf(rootCtx, err, "Error Creating Kubernetes Client")
			os.Exit(1)
		}
		wg := &sync.WaitGroup{}
		kubesecretpipe.Spawn(rootCtx, wg, clientset, config)
		wg.Wait()
	},
}

var flagConfigFilePath *string
var flagInClusterCreds *bool
var flagKubeConfigFile *string

var _ = onInit(func() {
	rootCmd.AddCommand(runCmd)
	flagConfigFilePath = runCmd.PersistentFlags().StringP("config-file", "c", "", "The YAML configfile")
	runCmd.MarkPersistentFlagFilename("config-file", "yaml")
	runCmd.MarkPersistentFlagRequired("config-file")

	isKubernetes := hasKubernetesServiceAccount()

	flagInClusterCreds = runCmd.PersistentFlags().Bool("in-cluster-creds", isKubernetes, "When in Kubernetes, automatically use the pod service account to connect to the cluster.")

	flagKubeConfigFile = runCmd.PersistentFlags().String("kubeconfig-creds", "", "Path to a kubeconfig file to use to connect to the cluster")
})
