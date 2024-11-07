# Setup

## Local Setup with Kind

### Prerequisites

To run this project locally, you'll need to install [finch][finch], [Go][go], [kubectl][kubectl], [kind][kind], and [kubebuilder][kubebuilder] (if creating/modifying CRDs).

[finch]: https://github.com/runfinch/finch
[go]: https://go.dev/dl
[kubectl]: https://kubernetes.io/docs/tasks/tools/
[kind]: https://kind.sigs.k8s.io/
[kubebuilder]: https://book.kubebuilder.io/quick-start

### Kind

It's required to install `kind` version `v0.24.0` (or later), in order to be compatible with Kubernetes `v1.31` and Finch.

To install `kind` you can use `go` or the package manager available in your O.S. We'll be covering [`brew`](https://brew.sh/) in this example, but you can find more installation options [here](https://kind.sigs.k8s.io/docs/user/quick-start/#installation).

#### Go

```bash
go install sigs.k8s.io/kind@v0.24.0
# ensure $GOPATH/bin is in your $PATH
kind --version
# kind version 0.24.0
```

#### Homebrew

```bash
brew install kind
kind --version
# kind version 0.24.0
```

### Finch

Use `brew` to install Finch, other installation options can be found [here](https://github.com/runfinch/finch?tab=readme-ov-file#installing-finch).

```bash
brew install --cask finch
finch --version
# finch version v1.4.1
```

After the installation, it's required that Finch VM is initialized. Then ensure you have a `finch` VM build.

```bash
finch vm init
# INFO[0000] Initializing and starting Finch virtual machine... 
# INFO[0049] Finch virtual machine started successfully   
finch vm status
# Running
```

If already have a VM initialized in the past, you may need to just start it.

```bash
finch vm start
# INFO[0000] Starting existing Finch virtual machine...   
# INFO[0019] Finch virtual machine started successfully   
finch vm status
# Running
```

## Local Quickstart

1. Clone this repository to your local environment or IDE.

    ```bash
    git clone https://github.com/awslabs/cedar-access-control-for-k8s.git
    cd cedar-access-control-for-k8s
    ```

2. For an optional local build of the binaries, you can run:

    ```bash
    make build
    ```

    If you encounter an error related to `goproxy` like the one below, try exporting the following environment variable.

    ```bash
    # go: sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0: sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0: Get "https://proxy.golang.org/sigs.k8s.io/controller-tools/cmd/controller-gen/@v/v0.14.0.info": dial tcp: lookup proxy.golang.org: i/o timeout

    export GOPROXY=direct
    ```

3. Start the Kind cluster. This will build the webhook image, the Kind image, and create the Kind cluster. This cluster is configured to authorize and validate requests via the Cedar webhook:

   ```bash
   make kind
   ```

4. (Optional) Create additional policies. There's an example in `demo/authorization-policy.yaml` that is auto-created, but feel free to modify it or create more

   ```bash
   # edit demo/authorization-policy.yaml
   kubectl apply -f demo/authorization-policy.yaml
   ```

5. Generate a `kubeconfig` for a test user. The user has the name `test-user` with the group `test-group`.

    ```bash
    make test-user-kubeconfig
    # Lookup the username of the test user
    KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl auth whoami
    # ATTRIBUTE   VALUE
    # Username    test-user
    # Groups      [viewers test-group system:authenticated]
    ```

6. Now you can make requests! You'll need to use the generated `kubeconfig` in `./mount/test-user-kubeconfig.yam` created in the previous step. Your default `kubeconfig` (`~/.kube/config`) will be autoconfigured by kind with a cluster administrator identity, so `kubectl` without specifying a `kubeconfig` should always just work.

    Let's test both `kubeconfig` files to validate if our setup is working.

    Try getting resources like Pods and Nodes.

    ```bash
    KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get pods --all-namespaces # allowed
    KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get nodes # denied
    ```

    As `cluster-admin`, list Secrets and Nodes.

    ```bash
    kubectl get nodes
    kubectl get secrets --show-labels
    ```

    Try listing Secrets with the `test-user`.

    ```bash
    KUBECONFIG=./mount/test-user-kubeconfig.yaml kubectl get secrets # denied
    ```

7. Try out the scenarios on the [Demo](./Demo.md) for different policies for authorization access and admission controls.

## Cleanup

For tearing down the Kind cluster.

```bash
make clean-kind
```

And to cleanup the Finch VM.

```bash
finch vm stop                                                                                               
# INFO[0000] Stopping existing Finch virtual machine...   
# INFO[0005] Finch virtual machine stopped successfully   
finch vm remove
# INFO[0000] Removing existing Finch virtual machine...   
# INFO[0000] Finch virtual machine removed successfully   
```
