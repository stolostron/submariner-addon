# SubmarinerConfig

SubmarinerConfig is a namespace-scoped API which can build the cluster environment automatically to meet the prerequisites for running Submariner. The User can also customize the configurations used in Submariner by SubmarinerConfig like Cable Driver, IPSec IKE ports etc.

The SubmarinerConfig should be created in the managed cluster namespace.

## Limitation

SubmarinerConfig can only support OCP an AWS at the current stage. The other Cloud Platforms will be supported in the future.

## Use Cases

1. As a user, I have prepared my Submariner cluster environment, but I used myself configurations, for example, I set the `IPSecIKEPort` to 501 and set the `IPSecNATTPort` to 4501. So I should create a SubmarinerConfig with my configurations.

    ```yaml
    apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
    kind: SubmarinerConfig
    metadata:
        name: <config-name>
        namespace: <managed-cluster-namespace>
    spec:
        IPSecIKEPort: <IPSec IKE Port>
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
        IPSecIKEPort: <IPSec IKE Port>
        IPSecNATTPort: <IPSec NAT-T Port>
        credentialsSecret:
            name: <cloud-provider-credential-secret-name>
    ```

    The format of credentials Secret is the same as the one used to provision the cluster by ACM.

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
