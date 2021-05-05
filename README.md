# Remote Velero

## Overview

Remote Velero is a fork of Velero. Velero lets you:

* Take backups of your cluster and restore in case of loss.
* Migrate cluster resources to other clusters.
* Replicate your production cluster to development and testing clusters.

Velero consists of:

* A server that runs on your cluster.
* A command-line client that runs locally.

Remote Velero is the same as above with the addition of

* The ability to back up from a remote cluster (called the source cluster).
* The ability to restore to a remote cluster (called the destination cluster).

## How to Use Remote Velero (OPTION 1: One Velero using Two Different Remote Clusters)

Here are instructions for setting up one Velero installation on a local service cluster.
Service account credentials enable Velero to connect to two different remote clusters:
one for the source (backup) cluster, and the other for the destination (restore) cluster.

1. Build the Remote Velero binary to use as client.

    * From the root of the Remote Velero project, run

      ```bash
      make local
      ```

    * The binary will be generated in a subdirectory of `<Remote Velero project root>/_output/bin/`
    * Remote Velero binary releases are [available for download](https://github.com/codegold79/remote-velero/releases)

1. Provide remote cluster credentials

    * Create a namespace that Velero will be installed to (default is `velero`)

      ```bash
      kubectl create namespace velero
      ```

    * **Note:** It is important to install these secrets before Velero is installed. If you've already installed Velero, delete using

    ```bash
    kubectl delete namespace velero; kubectl delete crd -l component=velero
    ```

    * Modify and apply secrets, one each for your two remote clusters (one for source/backup and another for destination/restore)

        * The secret for the remote source cluster must be named `srccluster`.
        * The secret for the remote destination cluster must be named `destcluster`.
        * Both secrets must be in the namespace that Velero was installed in.
        * Provide cluster credentials in one of two ways:
          1. The data in each secret must contain the host URL associated with the `host` key, and the service account token for the `sa-token` key.
          2. Or, provide the host URL for the `host` key and the contents of the cluster's `kubeconfig` file in the `kubeconfig` key.
        * In the secret, you can optionally provide an HTTPS proxy url to use under the `https_proxy` key.
        * See an example secret manifest for a remote source cluster in [remote-velero/service-acct-creds/src-cluster-cred-example.yaml](remote-velero/service-acct-creds/src-cluster-cred-example.yaml).

        ```yaml
        apiVersion: v1
        kind: Secret
        metadata:
        name: <srccluster or destcluster>
        namespace: <namespace where Velero is installed>
        type: Opaque
        data:
          host: <base64 encoded host URL>
          sa-token: <base64 encoded service account token here>
          kubeconfig: <base64 encoded kubeconfig file contents here>
          https_proxy: <base64 encoded https proxy URL here>
        ```

1. Install Remote Velero
    **Note:** this step must be done after secrets for the remote cluster credentials are applied.

    * Set up your BSL and credentials per normal Velero operation.
    * Use client binary to install Remote Velero, pointing to a Remote Velero image.

    ```bash
    export VERSION=<version number> export IMAGE=projects.registry.vmware.com/tanzu_migrator/remote-velero
    vel install \
    --features=EnableAPIGroupVersions \
    --provider aws \
    --plugins velero/velero-plugin-for-aws:v1.2.0 \
    --use-volume-snapshots=false \
    --bucket velero \
    --prefix veldat \
    --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://<address-to-bsl>:9000 \
    --secret-file /path/to/secret/file/for/reading/backup-storage-location \
    --image $IMAGE:$VERSION
    ```

    * **Note:** Instead of downloading the Velero client binary, you can build your own image with `make container`.
    * To ensure the server is now pointing to the correct remote clusters, look at the velero deployment logs and look for a message similar to the following:

    ```bash
    level=info msg="Server is using source cluster at https://example.servicemesh.biz:6443." 
    level=info msg="Server is using namespace velero."
    level=info msg="Server is using destination cluster at https://example.us-east-2.elb.amazonaws.com:443."
    ```

1. Run backup of a namespace on a remote source cluster.

    ```bash
    velero backup create backup-1 --include-namespaces example
    ```

1. Run restore of a namespace on a remote destination cluster.

    ```bash
    velero restore create restore-1 --from-backup backup-1
    ```

## How to Use Remote Velero (OPTION 2: Multiple Namespaces, each with Velero Installs. Each Velero Connects to One Remote Cluster)

Here are instructions for setting up two namespaces, each with a Velero installation. Each Velero installation connects to a single
remote cluster. You can have as many namespaces and Veleros that a single cluster can hold.

Instructions are very similar to Option 1, one velero for two remote clusters. Some build details have been omitted here.

1. Get latest Remote Velero client version from [available binary releases](https://github.com/codegold79/remote-velero/releases).

1. Provide remote cluster credentials before installing Velero in the same namespace.

    * Create a namespace that Velero will be installed to, e.g. "src-velero".

      ```bash
      kubectl create namespace src-velero
      ```
    * Create additional namespace(s) that other Veleros will be installed to, e.g. "dest-velero".

      ```bash
      kubectl create namespace dest-velero
      ```

    * Modify the example secret at [remote-velero/service-acct-creds/remote-cluster-cred-example.yaml](remote-velero/service-acct-creds/remote-cluster-cred-example.yaml) and apply secrets in their respective namespaces.

    **Note: Each namespace has a single secret named "remotecluster".**

    * The secret added to each namespace must be named `remotecluster`.
    * Provide cluster credentials for each remote cluster in one of two ways:
        1. The data in each secret must contain the host URL associated with the `host` key, and the service account token for the `sa-token` key.
        2. Or, provide the host URL for the `host` key and the contents of the cluster's `kubeconfig` file in the `kubeconfig` key.
    * In the secret, you can optionally provide an HTTPS proxy url to use under the `https_proxy` key.
    * See an example secret manifest for a remote source cluster in [remote-velero/service-acct-creds/src-cluster-cred-example.yaml](remote-velero/service-acct-creds/remote-cluster-cred-example.yaml).

        ```yaml
        apiVersion: v1
        kind: Secret
        metadata:
        name: remotecluster
        namespace: <namespace where Velero is installed>
        type: Opaque
        data:
        host: <base64 encoded host URL>
        sa-token: <base64 encoded service account token here>
        kubeconfig: <base64 encoded kubeconfig file contents here>
        https_proxy: <base64 encoded https proxy URL here>
        ```

1. Install Remote Velero in Every Namespace
    **Note:** this step must be done after secrets for the remote cluster credentials are applied. You can restart Velero pod to see changes to secrets.

    * Set up your BSL and credentials per normal Velero operation. Be sure to include the `--namespace src-velero` flag with every `velero` client command.
    * Install Remote Velero, pointing to a Remote Velero server image. Be sure to include the `--namespace src-velero` flag with the `velero install` client command.

    ```bash
    export VERSION=<version> export IMAGE=projects.registry.vmware.com/tanzu_migrator/remote-velero
    vel install \
    --features=EnableAPIGroupVersions \
    --provider aws \
    --plugins velero/velero-plugin-for-aws:v1.2.0 \
    --use-volume-snapshots=false \
    --bucket velero \
    --prefix veldat \
    --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://<address-to-bsl>:9000 \
    --secret-file /path/to/secret/file/for/reading/backup-storage-location \
    --image $IMAGE:$VERSION \
    --namespace src-velero
    ```

    * To ensure the server is now pointing to the correct remote clusters, look at the velero deployment logs and look for a message similar to the following:

    ```bash
    level=info msg="Server is using source cluster at https://example.servicemesh.biz:6443." 
    level=info msg="Server is using namespace velero." logSource="pkg/cmd/server/server.go:408"
    level=info msg="Server is using destination cluster at https://example.us-east-2.elb.amazonaws.com:443."
    ```

1. Run backup of a namespace on a remote source cluster.

    ```bash
    velero backup create backup-1 --include-namespaces example --namespace src-velero
    ```

1. Run restore of a namespace on a remote destination cluster.

    ```bash
    velero restore create restore-1 --from-backup backup-1 --namespace dest-velero
    ```
