/*
Copyright 2017, 2019 the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"net/http"
	"net/url"
	"os"

	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
)

const (
	srcClusterSecretName    = "srccluster"
	destClusterSecretName   = "destcluster"
	remoteClusterSecretName = "remotecluster"
)

// Factory knows how to create a VeleroClient and Kubernetes client.
type Factory interface {
	// BindFlags binds common flags (--kubeconfig, --namespace) to the passed-in FlagSet.
	BindFlags(flags *pflag.FlagSet)

	// Client returns a VeleroClient. It uses the following priority to specify the cluster
	// configuration: --kubeconfig flag, KUBECONFIG environment variable, in-cluster configuration.
	Client() (clientset.Interface, error)
	// SourceClient returns a VeleroClient. The client uses the config returned by
	// the SourceClientConfig() method.
	SourceClient() (clientset.Interface, error)
	// DestinationClient returns a VeleroClient. The client uses the config returned by
	// the DestinationClient() method.
	DestinationClient() (clientset.Interface, error)
	// KubeClient returns a Kubernetes client. It uses the following priority to specify the cluster
	// configuration: --kubeconfig flag, KUBECONFIG environment variable, in-cluster configuration.
	KubeClient() (kubernetes.Interface, error)
	// SourceKubeClient returns a Kubernetes client. It uses information in a
	// user-provided secret to connect to and gain access to a remote cluster.
	SourceKubeClient() (kubernetes.Interface, error)
	// DestinationKubeClient returns a Kubernetes client. It uses information
	// in a user-provided secret to connect to and gain access to a remote cluster.
	DestinationKubeClient() (kubernetes.Interface, error)
	// DynamicClient returns a Kubernetes dynamic client. It uses the following priority to specify the cluster
	// configuration: --kubeconfig flag, KUBECONFIG environment variable, in-cluster configuration.
	DynamicClient() (dynamic.Interface, error)
	// SourceDynamicClient returns a Kubernetes dynamic client.
	SourceDynamicClient() (dynamic.Interface, error)
	// DestinationDynamicClient returns a Kubernetes dynamic client.
	DestinationDynamicClient() (dynamic.Interface, error)
	// KubebuilderClient returns a Kubernetes dynamic client. It uses the following priority to specify the cluster
	// KubebuilderClient returns a client for the controller runtime framework. It adds Kubernetes and Velero
	// types to its scheme. It uses the following priority to specify the cluster
	// configuration: --kubeconfig flag, KUBECONFIG environment variable, in-cluster configuration.
	KubebuilderClient() (kbclient.Client, error)

	// SetBasename changes the basename for an already-constructed client.
	// This is useful for generating clients that require a different user-agent string below the root `velero`
	// command, such as the server subcommand.
	SetBasename(string)
	// SetClientQPS sets the Queries Per Second for a client.
	SetClientQPS(float32)
	// SetClientBurst sets the Burst for a client.
	SetClientBurst(int)

	// ClientConfig returns a rest.Config struct used for client-go clients.
	ClientConfig() (*rest.Config, error)
	// SourceClientConfig returns a rest.Config struct used for client-go clients.
	SourceClientConfig() (*rest.Config, error)
	// DestinationClientConfig returns a rest.Config struct used for client-go clients.
	DestinationClientConfig() (*rest.Config, error)

	// SrcClusterHost returns the URL of the remote cluster that will be back up.
	SrcClusterHost() string
	// DestClusterHost returns the URL of the remote cluster to restore to.
	DestClusterHost() string

	// Namespace returns the namespace which the Factory will create clients for.
	Namespace() string

	// HttpProxy...
	HttpProxy() string
	// HttpsProxy...
	HttpsProxy() string
}

type factory struct {
	flags           *pflag.FlagSet
	kubeconfig      string
	kubecontext     string
	srcClusterHost  string
	destClusterHost string
	baseName        string
	namespace       string
	clientQPS       float32
	clientBurst     int
	httpsProxy      string
	httpProxy       string
}

// NewFactory returns a Factory.
func NewFactory(baseName string, config VeleroConfig) Factory {
	f := &factory{
		flags:    pflag.NewFlagSet("", pflag.ContinueOnError),
		baseName: baseName,
	}

	f.namespace = os.Getenv("VELERO_NAMESPACE")
	if config.Namespace() != "" {
		f.namespace = config.Namespace()
	}

	// We didn't get the namespace via env var or config file, so use the default.
	// Command line flags will override when BindFlags is called.
	if f.namespace == "" {
		f.namespace = velerov1api.DefaultNamespace
	}

	f.flags.StringVar(&f.kubeconfig, "kubeconfig", "", "Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration")
	f.flags.StringVarP(&f.namespace, "namespace", "n", f.namespace, "The namespace in which Velero should operate")
	f.flags.StringVar(&f.kubecontext, "kubecontext", "", "The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)")
	f.flags.StringVar(&f.httpsProxy, "httpsproxy", f.httpsProxy, "The proxy to use for https connections")
	// TODO: httpproxy is a flag, but is not currently used.
	f.flags.StringVar(&f.httpProxy, "httpproxy", f.httpProxy, "The proxy to use for http connections")

	return f
}

func (f *factory) BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(f.flags)
}

func (f *factory) ClientConfig() (*rest.Config, error) {
	return Config(f.kubeconfig, f.kubecontext, f.baseName, f.clientQPS, f.clientBurst)
}

type serviceAcctCreds struct {
	host       string
	saToken    string
	kubeconfig string
	httpsProxy string
}

// SourceClientConfig will return return a rest config built using the
// credentials information in a user-provided secret.
func (f *factory) SourceClientConfig() (*rest.Config, error) {
	// First see if there are remote cluster service account credentials saved.
	srcCreds, err := f.serviceAcctCredsFromSecret(
		remoteClusterSecretName,
		f.namespace,
	)
	if err != nil {
		return nil, err
	}

	// Try getting the source cluster service account creds next.
	if (srcCreds == serviceAcctCreds{}) {
		srcCreds, err = f.serviceAcctCredsFromSecret(
			srcClusterSecretName,
			f.namespace,
		)
		if err != nil {
			return nil, err
		}
	}

	if (srcCreds != serviceAcctCreds{}) {
		f.srcClusterHost = srcCreds.host

		// Use kubeconfig if provided. Kubeconfig must provide TLS certificate
		// data.
		if srcCreds.kubeconfig != "" {
			return f.restConfigWithKubeConfig(srcCreds)
		}

		// Passing in the SA token assumes TLS insecure is true. Only used if
		// kubeconfig has not been provided.
		return f.restConfigWithSAToken(srcCreds)
	}

	// No service account credentials were found for source cluster in secret.
	// Use local cluster kubecontext.
	return Config(f.kubeconfig, f.kubecontext, f.baseName, f.clientQPS, f.clientBurst)
}

// DestinationClientConfig will return return a rest config built using the
// credentials information in a user-provided secret.
func (f *factory) DestinationClientConfig() (*rest.Config, error) {
	// First see if there are remote cluster service account credentials saved.
	destCreds, err := f.serviceAcctCredsFromSecret(
		remoteClusterSecretName,
		f.namespace,
	)
	if err != nil {
		return nil, err
	}

	// Try getting the destination cluster service account creds next.
	if (destCreds == serviceAcctCreds{}) {
		destCreds, err = f.serviceAcctCredsFromSecret(
			destClusterSecretName,
			f.namespace,
		)
		if err != nil {
			return nil, err
		}
	}

	if (destCreds != serviceAcctCreds{}) {
		f.destClusterHost = destCreds.host

		// Use kubeconfig if provided. Kubeconfig must provide TLS certificate
		// data.
		if destCreds.kubeconfig != "" {
			return f.restConfigWithKubeConfig(destCreds)
		}

		// Passing in the SA token assumes TLS insecure is true. Only used if
		// kubeconfig has not been provided.
		return f.restConfigWithSAToken(destCreds)
	}

	// No service account credentials were found for source cluster in secret.
	// Use local cluster kubecontext.
	return Config(f.kubeconfig, f.kubecontext, f.baseName, f.clientQPS, f.clientBurst)
}

func (f *factory) restConfigWithSAToken(creds serviceAcctCreds) (*rest.Config, error) {
	config := rest.Config{
		Host:            creds.host,
		BearerToken:     creds.saToken,
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
		Burst:           1000,
		QPS:             100,
	}

	if f.httpsProxy != "" {
		setTransportProxy(&config, f.httpsProxy)
	}

	return &config, nil
}

func (f *factory) restConfigWithKubeConfig(creds serviceAcctCreds) (*rest.Config, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(creds.kubeconfig))
	if err != nil {
		return nil, err
	}

	if f.httpsProxy != "" {
		setTransportProxy(config, f.httpsProxy)
	}
	return config, nil
}

func setTransportProxy(config *rest.Config, proxy string) {
	config.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		transport := rt.(*http.Transport)
		proxyURL, _ := url.Parse(proxy)
		transport.Proxy = http.ProxyURL(proxyURL)
		return transport
	})
}

func (f *factory) Client() (clientset.Interface, error) {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}

	veleroClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return veleroClient, nil
}

func (f *factory) SourceClient() (clientset.Interface, error) {
	clientConfig, err := f.SourceClientConfig()
	if err != nil {
		return nil, err
	}

	veleroClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return veleroClient, nil
}

func (f *factory) DestinationClient() (clientset.Interface, error) {
	clientConfig, err := f.DestinationClientConfig()
	if err != nil {
		return nil, err
	}

	veleroClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return veleroClient, nil
}

func (f *factory) KubeClient() (kubernetes.Interface, error) {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return kubeClient, nil
}

func (f *factory) SourceKubeClient() (kubernetes.Interface, error) {
	clientConfig, err := f.SourceClientConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return kubeClient, nil
}

func (f *factory) DestinationKubeClient() (kubernetes.Interface, error) {
	clientConfig, err := f.DestinationClientConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return kubeClient, nil
}

func (f *factory) DynamicClient() (dynamic.Interface, error) {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return dynamicClient, nil
}

func (f *factory) SourceDynamicClient() (dynamic.Interface, error) {
	clientConfig, err := f.SourceClientConfig()
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return dynamicClient, nil
}

func (f *factory) DestinationDynamicClient() (dynamic.Interface, error) {
	clientConfig, err := f.DestinationClientConfig()
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return dynamicClient, nil
}

func (f *factory) KubebuilderClient() (kbclient.Client, error) {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	velerov1api.AddToScheme(scheme)
	k8scheme.AddToScheme(scheme)
	apiextv1beta1.AddToScheme(scheme)
	kubebuilderClient, err := kbclient.New(clientConfig, kbclient.Options{
		Scheme: scheme,
	})

	if err != nil {
		return nil, err
	}

	return kubebuilderClient, nil
}

func (f *factory) SetBasename(name string) {
	f.baseName = name
}

func (f *factory) SetClientQPS(qps float32) {
	f.clientQPS = qps
}

func (f *factory) SetClientBurst(burst int) {
	f.clientBurst = burst
}

func (f *factory) Namespace() string {
	return f.namespace
}

func (f *factory) SrcClusterHost() string {
	return f.srcClusterHost
}

func (f *factory) DestClusterHost() string {
	return f.destClusterHost
}

// serviceAccountCredsFromSecret looks for service account credentials from a secret
// identified by the secret's name and namespace.
func (f *factory) serviceAcctCredsFromSecret(secretName, secretNS string) (serviceAcctCreds, error) {
	client, err := f.KubeClient()
	if err != nil {
		return serviceAcctCreds{}, err
	}

	secrets, err := client.CoreV1().Secrets(secretNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return serviceAcctCreds{}, err
	}

	var saCreds serviceAcctCreds
	for _, item := range secrets.Items {
		if item.Name == secretName {
			saCreds = serviceAcctCreds{
				host:       string(item.Data["host"]),
				saToken:    string(item.Data["sa-token"]),
				kubeconfig: string(item.Data["kubeconfig"]),
				httpsProxy: string(item.Data["https_proxy"]),
			}

			if f.httpsProxy == "" && saCreds.httpsProxy != "" {
				f.httpsProxy = saCreds.httpsProxy
			}

			return saCreds, nil
		}
	}

	// No service account credentials for remote cluster found in secret.
	return serviceAcctCreds{}, nil
}

// HttpProxy is a getter for HTTP Proxy address.
func (f *factory) HttpProxy() string {
	return f.httpProxy
}

// HttpsProxy is a getter for HTTPS Proxy address.
func (f *factory) HttpsProxy() string {
	return f.httpsProxy
}
