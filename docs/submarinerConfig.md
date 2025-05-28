# SubmarinerConfig

SubmarinerConfig is a namespace-scoped API which can build the cluster environment automatically to meet the prerequisites for running Submariner. The User can also customize the configurations used in Submariner by SubmarinerConfig like Cable Driver, IPSec NAT-T ports etc.

The SubmarinerConfig should be created in the managed cluster namespace.

## Limitation

SubmarinerConfig can support OCP on AWS, GCP or VMware vSphere at the current stage. The other Cloud Platforms will be supported in the future.

## Use Cases

1. As a user, I have prepared my Submariner cluster environment, but I used myself configurations, for example, I set the `IPSecNATTPort` to 4501. So I should create a SubmarinerConfig with my configurations.

    ```yaml
    apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
    kind: SubmarinerConfig
    metadata:
        name: <config-name>
        namespace: <managed-cluster-namespace>
    spec:
        IPSecNATTPort: <IPSec NAT-T Port>
        ...
    ```

2. As a user, I have not prepared my submariner cluster environment, I want submariner-addon to help me prepare the environment, so I need create a SubmarinerConfig with my configurations and cloud provider credentials.

    ```yaml
    apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
    kind: SubmarinerConfig
    metadata:
        name: <config-name>
        namespace: <managed-cluster-namespace>
    spec:
        IPSecNATTPort: <IPSec NAT-T Port>
        credentialsSecret:
            name: <cloud-provider-credential-secret-name>
    ```

    For ACM, the format of credentials Secret is

    ```yaml 
    apiVersion: v1
    kind: Secret
    metadata:
        name: <cloud-provider-credential-secret-name>
        namespace: <managed-cluster-namespace>
    type: Opaque
    data:
        aws_access_key_id: <aws-access-key-id>
        aws_secret_access_key:    <aws-secret-access-key>
    ```

    It is the same as the one used to provision the cluster by ACM.

    For GCP, the format of credentials Secret is

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
        name: <cloud-provider-credential-secret-name>
        namespace: <managed-cluster-namespace>
    type: Opaque
    data:
        osServiceAccount.json: <gcp-service-account-json-file>
    ```

    It is the same as the one used to provision the cluster by GCP.

3. As a user, I want to change the default instance type of gateways on AWS

    ```yaml
    apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
    kind: SubmarinerConfig
    metadata:
        name: <config-name>
        namespace: <managed-cluster-namespace>
    spec:
        gatewayConfig:
          instanceType: <aws-instance-type>
        ...
    ```

4. As a user, I want to specfiy the number of gateways

    ```yaml
    apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
    kind: SubmarinerConfig
    metadata:
        name: <config-name>
        namespace: <managed-cluster-namespace>
    spec:
        gatewayConfig:
          gateways: <gateway-numbers>
        ...
    ```

5. As a user, I want to change the default definition of submariner-operator subscription

   ```yaml
    apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
    kind: SubmarinerConfig
    metadata:
        name: <config-name>
        namespace: <managed-cluster-namespace>
    spec:
        subscriptionConfig:
          source: <submariner-operator-source>
          sourceNamespace: <submariner-operator-source-namespace>
          channel: <submariner-operator-channel>
          startingCSV: <submariner-operator-staring-csv>
        ...
    ```

6. As a user, I want to override the image pull specs

   ```yaml
    apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
    kind: SubmarinerConfig
    metadata:
        name: <config-name>
        namespace: <managed-cluster-namespace>
    spec:
        imagePullSpecs:
          submarinerImagePullSpec: <submariner-image-pull-spec>
          lighthouseAgentImagePullSpec: <lighthouse-agent-image-pull-spec>
          lighthouseCoreDNSImagePullSpec: <lighthouse-coredns-image-pull-spec>
          submarinerRouteAgentImagePullSpec: <submariner-route-image-pull-spec>
          metricsProxyImagePullSpec: <metrics-proxy-image-pull-spec>
          nettestImagePullSpec: <nettest-image-pull-spec>
        ...
    ```
