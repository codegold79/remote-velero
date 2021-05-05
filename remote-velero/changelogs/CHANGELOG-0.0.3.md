# Changes

- Enable passing in of httpsProxy via secret.
- Enable passing in of httpsProxy and httpProxy via Velero install flags. Proxy flags override secrets. HttpProxy is not yet being used.
- Enable passing in of a kubeconfig that makes use of TLS via secret. Requires TLS certificate data.
