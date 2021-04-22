# Remote Velero

## Overview

Remote Velero is a fork of Velero. Velero lets you:

* Take backups of your cluster and restore in case of loss.
* Migrate cluster resources to other clusters.
* Replicate your production cluster to development and testing clusters.

Velero consists of:

* A server that runs on your cluster
* A command-line client that runs locally

Remote Velero is the same as above with the addition of

* The ability to back up from a remote cluster (called the source cluster)
* The ability to restore to a remote cluster (called the destination cluster)

## How to Use Remote Velero

1. Build the Remote Velero binary to use as client.

    * From the root of the Remote Velero project, run

      ```bash
      make local
      ```

    * The binary will be generated in a subdirectory of `<Remote Velero project root>/_output/bin/`
    * A binary with version 0.0.1 of Remote Velero has been included in [remote-velero/binaries](remote-velero/binaries)

1. Provide remote cluster credentials

    * Create a namespace that Velero will be installed to (default is `velero`)

      ```bash
      kubectl create namespace velero
      ```

    * **Note:** It is important to install these secrets before Velero is installed. If you've already installed Velero, delete using

    ```bash
    kubectl delete namespace velero; kubectl delete crd -l component=velero
    ````

    * Adjust and apply secrets, one each for your two remote clusters (one for source/backup and another for destination/restore)

        * The secret for the remote source cluster must be named `srccluster`.
        * The secret for the remote destination cluster must be named `destcluster`.
        * Both secrets must be in the `velero` namespace.
        * The data in each secret must contain the host URL associated with the `host` key, the service account name for the `sa-name` key, the service account namespace for the `sa-namesapce` key, and the service account token for the `sa-token` key.
        * See an example secret manifest in [remote-velero/service-acct-creds/remote-cluster-cred.yaml](remote-velero/service-acct-creds/remote-cluster-cred.yaml) for a concrete example.

        ```yaml
        apiVersion: v1
        kind: Secret
        metadata:
        name: srccluster
        namespace: velero
        type: Opaque
        data:
        host: <base64 encoded host URL>
        sa-name: <base64 encoded service account name>
        sa-namespace: <base64 encoded service account namespace>
        sa-token: <base64 encoded service account token here>
        ```

1. Install Remote Velero

    **Note:** this step must be done after secrets for the remote cluster credentials are applied.

    * Set up your BSL and credentials per normal Velero operation.
    * Use client binary to install Remote Velero, pointing to a Remote Velero image.

    ```bash
    export VERSION=0.0.1 export IMAGE=projects.registry.vmware.com/tanzu_migrator/remote-velero
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

    * **Note:** You can use your own image if you'd like. Create an image with `make container`.
    * To ensure the server is now pointing to the correct remote clusters, look at the velero deployment logs and look for a message similar to the following:

    ```bash
    level=info msg="Server is using source cluster at https://example.servicemesh.biz:6443." 
    level=info msg="Server is using namespace velero." logSource="pkg/cmd/server/server.go:408"
    level=info msg="Server is using destination cluster at https://example.us-east-2.elb.amazonaws.com:443."
    ```

1. Run backup of a namespace on a remote source cluster

    ```bash
    velero backup create backup-1 --include-namespaces example
    ```

1. Run restore of a namespace on a remote destination cluster

    ```bash
    velero restore create restore-1 --from-backup backup-1
    ```
