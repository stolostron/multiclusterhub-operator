# Disconnected Install on AWS

## Create AWS Environment

### Create VPC

For this example, we'll use the name `my-dc1` for resources related to this vpc.

Go to
[https://console.aws.amazon.com/vpc/home?region=us-east-1#vpcs:](https://console.aws.amazon.com/vpc/home?region=us-east-1#vpcs:)

Click "Create VPC"

- For "Resources to create", select "VPC, subnets, etc."
- For "Name tag auto-generation", enter `my-dc1`
- Change "Availability Zones" to 1.
- Expand "Customize AZs" and change availability zone to `us-east-1b`
- Ensure "Number of public subnets" is set to 1
- Expand "Customize public subnets CIDR blocks" and enter `10.0.0.0/24`
- Ensure "Number of private subnets" is set to 1
- Expand "Customize private subnets CIDR blocks" and enter `10.0.1.0/24`
- Ensure "NAT gateways" is set to None
- Ensure VPC endpoints is set to "S3 Gateway"
- Under "DNS options", check both

  - Enable DNS hostnames
  - Enable DNS resolution

Click "Create VPC"

Make a note of your VPC ID, such as `vpc-0554d61964eba9fa4`.

You will need this to filter resources in AWS later.

### Create security group for VPC

Go to
[https://console.aws.amazon.com/vpc/home?region=us-east-1#securityGroups:](https://console.aws.amazon.com/vpc/home?region=us-east-1#securityGroups:)

Click "Create security group"

- For "Name", use `my-dc1-sg`
- For "Description", use `External SSH and all internal traffic`
- For "VPC", select `my-dc1-vpc`
- Under Inbound Rules, click "Add rule"
    - Type: SSH
    - Source: 0.0.0.0/0
- Under Inbound Rules, click "Add rule"
    - Type: All traffic
    - Source: 10.0.0.0/16
- Under Outbound Rules, click "Add rule"
    - Type: All traffic
    - Destination: 0.0.0.0/0
- Under Tags, click "Add new tag"
    - Key: Name
    - Value: my-dc1-sg

### Create ec2 endpoint in VPC

Go to
[https://console.aws.amazon.com/vpc/home?region=us-east-1#Endpoints:](https://console.aws.amazon.com/vpc/home?region=us-east-1#Endpoints:)

Click "Create endpoint"

- For "Name tag", use `my-dc1-vpce-ec2`
- For "Service category", select "AWS services"
- Enter "ec2" in the "Filter services" search box
- Select the service "com.amazonaws.us-east-1.ec2"
- For "VPC", select your VPC
- Under "VPC", expand "Additional settings" and ensure "Enable DNS name" is checked.
- In "Subnets", check Availability Zone "us-east-1b"
- In "Subnets", for the "us-east-1b" AZ, select your private subnet
- In "Security groups", check the `my-dc1-sg` group
- In "Policy", ensure "Full access" is checked

Click "Create endpoint"

### Create ELB endpoint in VPC

Go to
[https://console.aws.amazon.com/vpc/home?region=us-east-1#Endpoints:](https://console.aws.amazon.com/vpc/home?region=us-east-1#Endpoints:)

Click "Create endpoint"

- For "Name tag", use `my-dc1-vpce-elb`
- For "Service category", select "AWS services"
- Enter "load" in the "Filter services" search box
- Select the service "com.amazonaws.us-east-1.elasticloadbalancing"
- For "VPC", select your VPC
- Under "VPC", expand "Additional settings" and ensure "Enable DNS name" is checked.
- In "Subnets", check Availability Zone "us-east-1b"
- In "Subnets", for the "us-east-1b" AZ, select your private subnet
- In "Security groups", check the `my-dc1-sg` group
- In "Policy", ensure "Full access" is checked

Click "Create endpoint"

### Create a Route 53 private hosted zone for your VPC

Go to
[https://console.aws.amazon.com/route53/v2/hostedzones](https://console.aws.amazon.com/route53/v2/hostedzones)

Click "Create hosted zone"

- For domain name, enter `my-dc1.dev01.red-chesterfield.com`
- Change the Type to "Private hosted zone"
- In the "Region" box, select us-east-1 region
- In the VPC ID box, select the vpc you created above

Click "Create hosted zone"

**Note:** Expand the "Hosted zone details" section and record the "Hosted zone ID"

`Z01757232ECSNQXVSUVVJ`

## Setup jumpbox EC2 instance in VPC

### Create jumpbox instance

Go to
[https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#Instances:v=3](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#Instances:v=3)

Click "Launch instances"

- Step 1: Choose an Amazon Machine Image (AMI)
    - Search for "Red Hat"
    - Under "Red Hat Enterprise Linux 8 (HVM), SSD Volume Type" ensure "64-bit (x86)" is selected
    - Click "Select" for "Red Hat Enterprise Linux 8 (HVM), SSD Volume Type"
- Step 2: Choose an instance type
    - Select "t3.medium"
    - Click "Next: Configure Instance Details"
- Step 3: Configure Instance Details
    - For "Network", select your VPC
    - For "Subnet", select your public subnet
    - For "Auto-assign Public IP", select "Enable"
    - Click "Next: Add Storage"
- Step 4: Add Storage
    - For the Root volume, change size to "100"
    - Click "Next: Add Tags"
- Step 5: Add Tags
    - Click "Add Tag"
    - Set Key to "Name"
    - Set Value to `my-dc1-jumpbox`
    - Click "Next: Configure Security Group"
- Step 6: Configure Security Group
    - Select "Select an existing security group"
    - Select the security group named `my-dc1-sg1`
    - Click "Review and Launch"
- Step 7: Review Instance Launch
    - Click "Launch"
- Dialog box "Select an exiting key pair or create a new key pair"
    - If you already have a key pair stored:
        - Select the key pair name
        - Check the box acknowledging you have access to the private key file
        - Click "Launch Instance"
    - If you do not have a key pair stored:
        - Select "Create a new key pair"
        - For "Key pair type", select RSA
        - For "Key pair name", enter `my-dc1-vpc-keypair`
        - Click "Download Key Pair" and save the `.pem` file
        - Click "Launch Instance"

Click "View Instances"

Find your instance by the name you gave it in the tag "Name".

Wait for it to be in the state "Running".

### SSH into jumpbox

```
ssh -i private-key.pem ec2-user@1.2.3.4
```

### Disable IPv6

- Edit `/etc/default/grub`

    ```
    sudo vi /etc/default/grub
    ```
    
- Add `ip6.disable=1` to the end of the `GRUB_CMDLINE_LINUX` entry.
- Save the file and exit.
- Generate the `grub.cfg` file.

    ```
    sudo grub2-mkconfig -o /boot/grub2/grub.cfg
    ```

- Reboot the jumpbox.

    ```
    sudo reboot
    ```

- Log back into the jump box.

    ```
    ssh -i private-key.pem ec2-user@1.2.3.4
    ```

### Verify IPv6 configuration

Run

```
ping -c 1 $HOSTNAME
```

You should see that the response is coming from your 10.0.0.x IPv4 address, such as

```
PING ip-10-0-1-161.ec2.internal (10.0.1.161) 56(84) bytes of data.
64 bytes from ip-10-0-1-161.ec2.internal (10.0.1.161): icmp_seq=1 ttl=64 time=0.068 ms
```

If this doesn't work, verify that you enabled DNS hostname and resolution in your VPC, and that you disabled IPv6 on the jumpbox as described above.

### Install required utilities

```
sudo yum -y install bind-utils jq podman zip
```

### Install oc command

```
curl -L -o oc.tgz https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/stable/openshift-client-linux.tar.gz
tar xf oc.tgz
sudo mv oc kubectl /usr/bin
rm oc.tgz README.md
```

### Install the opm command

```
curl -L -o opm.tgz https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/stable/opm-linux.tar.gz
tar xf opm.tgz
sudo mv opm /usr/bin
rm opm.tgz
```

### Install AWS CLI

```
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install
rm -rf aws awscliv2.zip
```

### Store your AWS credentials

```
mkdir "$HOME/.aws"
vi "$HOME/.aws/credentials"
```

The credentials file should look like this:

```
[default]
aws_access_key_id = AKI...
aws_secret_access_key = QjX...
```

## Setup Quay Mirror Registry

### Install a quay mirror registry instance

Docs are at
[https://docs.openshift.com/container-platform/4.9/installing/installing-mirroring-installation-images.html#installation-about-mirror-registry_installing-mirroring-installation-images](https://docs.openshift.com/container-platform/4.9/installing/installing-mirroring-installation-images.html#installation-about-mirror-registry_installing-mirroring-installation-images)

Run

```
curl -L -o mirror.tgz https://developers.redhat.com/content-gateway/file/pub/openshift-v4/clients/mirror-registry/1.0/mirror-registry.tar.gz
tar xf mirror.tgz
sudo ./mirror-registry install -v --quayHostname $HOSTNAME | tee quay-install.log
sudo rm *.tar mirror-registry mirror.tgz
```

### Extract quay password from install log

```
grep -oP "init, \K[^)]+" quay-install.log | tee quay_creds
```

### Add the quay mirror registry CA to the system trust store

```
sudo cp /etc/quay-install/quay-rootCA/rootCA.pem /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust extract
trust list | grep -C 2 "$HOSTNAME"
```

You should see output similar to this:

```
pkcs11:id=%83%C4%E2%B6%EA%D2%C5%21%45%38%15%A9%3A%19%59%E8%0A%AD%04%93;type=cert
    type: certificate
    label: ip-10-0-1-161.ec2.internal
    trust: anchor
    category: authority
```

### Download your OpenShift installation pull secret

- Go to:
  - [https://console.redhat.com/openshift/install/pull-secret](https://console.redhat.com/openshift/install/pull-secret)
- Select “Copy pull secret”.
- In your jumpbox, run

    ```
    vi auth.json
    ```
- Paste your pull secret into vi, save the file and exit.
- Make this file the default auth file, run

    ```
    export REGISTRY_AUTH_FILE=$HOME/auth.json
    echo export REGISTRY_AUTH_FILE=$REGISTRY_AUTH_FILE >> $HOME/.bash_profile
    ```

**Note:** Environment variables will also be stored in `$HOME/.bash_profile` so they'll be
recreated if you need to log out and back in to the jumpbox.

### Login to your mirror registry

This also compacts the auth.json file back down to a single line json file which is needed for openshift-install later.

```
podman login -u init -p $(cat quay_creds) $HOSTNAME:8443
jq -c . auth.json > auth.json.tmp && mv auth.json.tmp auth.json
```

### Mirror the OCP image repository

Go to this site to pick your version and architecture of OCP:

- [https://quay.io/repository/openshift-release-dev/ocp-release?tab=tags](https://quay.io/repository/openshift-release-dev/ocp-release?tab=tags)

For this example, we'll use `4.9.23-x86_64`.

Run:

```
export OCP_TAG=4.9.23-x86_64
export OCP_REPO=quay.io/openshift-release-dev/ocp-release
export LOCAL_REPO=$HOSTNAME:8443/ocp4/openshift4

cat <<EOF >>$HOME/.bash_profile
export OCP_TAG=$OCP_TAG
export OCP_REPO=$OCP_REPO
export LOCAL_REPO=$LOCAL_REPO
EOF

oc adm release mirror -a auth.json \
    --from=$OCP_REPO:$OCP_TAG \
    --to=$LOCAL_REPO \
    --to-release-image=$LOCAL_REPO/ocp-release:$OCP_TAG
```

### Prune and Mirror the Operator Catalog

Run

```
export INDEX_IMAGE=redhat-operator-index:v4.9
export INDEX_REPO=registry.redhat.io/redhat/$INDEX_IMAGE
export LOCAL_INDEX_REPO=$HOSTNAME:8443/ocp4/$INDEX_IMAGE
export LOCAL_OLM_REPO=$HOSTNAME:8443/ocp4/openshift4/olm-mirror

cat <<EOF >>$HOME/.bash_profile
export INDEX_IMAGE=$INDEX_IMAGE
export INDEX_REPO=$INDEX_REPO
export LOCAL_INDEX_REPO=$LOCAL_INDEX_REPO
export LOCAL_OLM_REPO=$LOCAL_OLM_REPO
EOF

opm index prune --from-index $INDEX_REPO --tag $LOCAL_INDEX_REPO \
    --packages advanced-cluster-management,multicluster-engine

podman push $LOCAL_INDEX_REPO

oc adm catalog mirror -a auth.json $LOCAL_INDEX_REPO $LOCAL_OLM_REPO

sudo rm -rf index_tmp_*
```

You should see a directory with a name similar to `manifests-redhat-operator-index-1646254114`.

Save a reference to this directory for later use:

```
export MANIFESTS=$HOME/manifests-redhat-operator-index-1646254114
echo export MANIFESTS=$MANIFESTS >> $HOME/.bash_profile
```

## Prepare to create OCP cluster

### Extract the OCP installer

```
oc adm release extract -a auth.json \
    --command=openshift-install $LOCAL_REPO/ocp-release:$OCP_TAG 
```

### Generate SSH key for cluster nodes

```
ssh-keygen -t ed25519 -N '' -f .ssh/id_cluster
eval "$(ssh-agent -s)"
ssh-add .ssh/id_cluster
```

### Create install files

Note: You may want to have a second terminal open on the jumpbox to be able to copy your auth.json file.

```
export INSTALL="$HOME/ocp-install"
echo export INSTALL="$INSTALL" >> $HOME/.bash_profile

mkdir -p "$INSTALL"

./openshift-install create install-config --dir "$INSTALL"
```

- Select the id_cluster ssh key you created in the previous step and press enter.
- Select `aws` and press enter.
- Select use-east-1 as the AWS region and press enter.
- Select dev01.red-chesterfield.com as the base domain and press enter.
- Enter a name for the cluster, such as `my-dc`, and press enter.
- Paste your pull secret from the `auth.json` file and press enter.

### Edit install-config.yaml

```
vi $INSTALL/install-config.yaml
```

The `credentialsMode` field should be set to `Passthrough`:

```
credentialsMode: Passthrough
```

The `compute` stanza should be

```
compute:
- architecture: amd64
  hyperthreading: Enabled
  name: worker
  platform:
    aws:
      region: us-east-1
      rootVolume:
        iops: 2000
        size: 500
        type: io1
      type: m5.xlarge
      zones:
      - us-east-1b
```

The `controlPlane` stanza should be

```
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform:
    aws:
      region: us-east-1
      rootVolume:
        iops: 4000
        size: 500
        type: io1
      type: m5.xlarge
      zones:
      - us-east-1b
  replicas: 3
```

The `networking` stanza should be

```
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  machineNetwork:
  - cidr: 10.0.0.0/16
  networkType: OpenShiftSDN
  serviceNetwork:
  - 172.30.0.0/16
```

The `platform` stanza should be similar to

```
platform:
  aws:
    region: us-east-1
    subnets:
    - subnet-0f375b056978b34a6
    hostedZone: Z01757232ECSNQXVSUVVJ
```

The subnet value should be the subnet ID for the private subnet in your VPC.

The hostedZone value should be the hosted zone ID for the Route 53 private
hosted zone you created when configuring your VPC.

The `publish` field should be

```
publish: Internal
```

Save your changes to the file.

### Add the trust bundle for your Quay mirror registry to the install files

Copy the contents of the file `/etc/quay-install/quay-rootCA/rootCA.pem` into the
`additionalTrustBundle` field of the install file.

```
cat <<EOF >>$INSTALL/install-config.yaml
additionalTrustBundle: |
$(sed 's/^/  /' /etc/quay-install/quay-rootCA/rootCA.pem)
EOF
```

### Add your quay mirror registry as an image content source to the install files

```
cat <<EOF >>$INSTALL/install-config.yaml
imageContentSources:
  - mirrors:
    - $LOCAL_REPO
    - $LOCAL_REPO/ocp-release
    source: quay.io/openshift-release-dev/ocp-release
  - mirrors:
    - $LOCAL_REPO
    - $LOCAL_REPO/ocp-release
    source: quay.io/openshift-release-dev/ocp-v4.0-art-dev
EOF
```

### Verify contents of install-config.yaml

Your `$INSTALL/install-config.yaml` file should be similar to this one:

```
apiVersion: v1
baseDomain: dev01.red-chesterfield.com
credentialsMode: Mint
compute:
- architecture: amd64
  hyperthreading: Enabled
  name: worker
  platform:
    aws:
      region: us-east-1
      rootVolume:
        iops: 2000
        size: 500
        type: io1
      type: m5.xlarge
      zones:
      - us-east-1b
  replicas: 3
controlPlane:
  architecture: amd64
  hyperthreading: Enabled
  name: master
  platform:
    aws:
      region: us-east-1
      rootVolume:
        iops: 4000
        size: 500
        type: io1
      type: m5.xlarge
      zones:
      - us-east-1b
  replicas: 3
metadata:
  creationTimestamp: null
  name: my-dc1
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  machineNetwork:
  - cidr: 10.0.0.0/16
  networkType: OpenShiftSDN
  serviceNetwork:
  - 172.30.0.0/16
platform:
  aws:
    region: us-east-1
    subnets:
    - subnet-0f375b056978b34a6
    hostedZone: Z01757232ECSNQXVSUVVJ
publish: Internal
pullSecret: '<PULL SECRET>'
sshKey: |
  ssh-ed25519 AAAAC3N... ec2-user@ip-10-0-0-20.ec2.internal
additionalTrustBundle: |
  -----BEGIN CERTIFICATE-----
  <QUAY MIRROR REGISTY TRUST BUNDLE>
  -----END CERTIFICATE-----
imageContentSources:
  - mirrors:
    - ip-10-0-0-20.ec2.internal:8443/ocp4/openshift4
    - ip-10-0-0-20.ec2.internal:8443/ocp4/openshift4/ocp-release
    source: quay.io/openshift-release-dev/ocp-release
  - mirrors:
    - ip-10-0-0-20.ec2.internal:8443/ocp4/openshift4
    - ip-10-0-0-20.ec2.internal:8443/ocp4/openshift4/ocp-release
    source: quay.io/openshift-release-dev/ocp-v4.0-art-dev
```

### Backup your install-config.yaml file

The `install-config.yaml` file will be deleted during cluster creation. Create a backup
to use if you need to reinstall the cluster or to verify how the cluster was created.

```
cp "$INSTALL/install-config.yaml" "$HOME/install.yaml.bak"
```

## Create OCP cluster

### Set up an alias to run oc with the new cluster credentials

```
alias oc="oc --kubeconfig=$INSTALL/auth/kubeconfig"
echo alias oc=\"oc --kubeconfig=$INSTALL/auth/kubeconfig\" >> $HOME/.bash_profile
```

### Run the installer to create your cluster

```
./openshift-install create cluster --dir $INSTALL --log-level=debug
```

Once you see this entry on the install logs:

```
INFO Waiting up to 40m0s for the cluster at https://api.my-dc1.dev01.red-chesterfield.com:6443 to initialize...
```

You will need to complete the "Configure cluster DNS" steps below before the 40 minutes are up
or the install will fail.

If you don't have your VPC ID, find it before starting the install to help ensure you
can quickly complete the cluster DNS configuration.

### (Optional) Monitor bootstrap installation

In the installer output, once all the resources have been created and the
installer is waiting for the kubernetes API to be available, you should see a
line like this in the log:

```
DEBUG bootstrap_ip = 10.0.1.203
```

You can watch the EC2 instances for the creation of the bootstrap node. It will have a
name similar to `my-dc1-ab123-bootstrap`.

Once the bootstrap node is running, you can ssh into the boostrap node from
the jumpbox by running

```
ssh -i .ssh/id_cluster core@10.0.1.203
```

On the bootstrap node, you can watch the cluster creation logs by running

```
journalctl -b -f -n all -u release-image.service -u bootkube.service
```


### (Optional) Monitor cluster initialization

In the installer output, once the installer is waiting for bootstrapping to complete,
you'll see a line like this in the output:

```
INFO Waiting up to 30m0s for bootstrapping to complete...
```

You can find the IP addresses of the master nodes by running:

```
$ oc get nodes
NAME                         STATUS   ROLES    AGE   VERSION
ip-10-0-1-19.ec2.internal    Ready    worker   12m   v1.22.3+b93fd35
ip-10-0-1-211.ec2.internal   Ready    worker   11m   v1.22.3+b93fd35
ip-10-0-1-231.ec2.internal   Ready    master   29m   v1.22.3+b93fd35
ip-10-0-1-244.ec2.internal   Ready    master   29m   v1.22.3+b93fd35
ip-10-0-1-43.ec2.internal    Ready    master   29m   v1.22.3+b93fd35
ip-10-0-1-67.ec2.internal    Ready    worker   11m   v1.22.3+b93fd35
```

You can then log in to one of the master nodes and watch the initialization logs by running:

```
ssh -i .ssh/id_cluster core@10.0.1.231
journalctl -b -f -n all -u kubelet.service -u crio.service
```

### Configure cluster DNS

In the installer output, once the bootstrap node has been destroyed, you should see a
line like this in the log:

```
INFO Waiting up to 40m0s for the cluster at https://api.my-dc1.dev01.red-chesterfield.com:6443 to initialize...
```

Once this appears, go to
[https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#LoadBalancers:sort=loadBalancerName](https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#LoadBalancers:sort=loadBalancerName)

In the search box, enter your VPC ID, such as `vpc-0554d61964eba9fa4`, to view just the
load balancers in your VPC.

Watch for a load balancer whose Type is `classic`. This load balancer will have a Name that is a
long hexidecimal value. Its DNS name will start with `internal-` followed by the Name.

When you see this load balancer (note that it will not have a "State"), follow these steps:

Make a note of the load balancer's name. You'll need to select it from a list in a following step.

Open another terminal to the jumpbox and run:

```
oc project openshift-ingress-operator
oc edit dnses.config/cluster
```

In the editor, under `spec`, remove the `privateZone` stanza. It should look similar to this:

```
apiVersion: config.openshift.io/v1
kind: DNS
metadata:
  creationTimestamp: "2022-03-07T14:37:25Z"
  generation: 2
  name: cluster
  resourceVersion: "24778"
  uid: 200759e9-3f21-4cb3-8802-ac1221e6ebf9
spec:
  baseDomain: my-dc1.dev01.red-chesterfield.com
status: {}
```

Save your changes.

Go to
[https://console.aws.amazon.com/route53/v2/hostedzones#](https://console.aws.amazon.com/route53/v2/hostedzones#)

Click on the hosted zone you created for your VPC. In this example, its name would be `my-dc1.dev01.red-chesterfield.com`

Click "Create record"

- For "Record name", enter `*.apps`
- For "Record type", ensure `A - Routes traffic to an IPv4 address and some AWS resources` is selected..
- For "Value", click the toggle to enable "Alias"
    - For "Choose endpoint", select `Alias to Application and Classic Load Balancer`
    - For "Choose region", select `US East (N. Virginia) [us-east-1]`
    - For "Choose load balancer", select your internal load balancer. Note: Its name in
      this list will be prefixed with `dualstack.`. For example:
      `dualstack.internal-a41440f20f34e42c194bcbf23206a4a0-1042296570.us-east-1.elb.amazonaws.com`

Click "Create Record"

Go back to watching the logs from the `openshift-install` command.

The install should complete successfully.

## Configure the OCP cluster after installation

### Disable the default OperatorHub sources

```
oc patch OperatorHub cluster --type json \
    -p '[{"op": "add", "path": "/spec/disableAllDefaultSources", "value": true}]'
```

### Setup OCP to use mirrored operator catalog

From the step "Prune and Mirror the Operator Catalog" above, you should have a directory
on your jumpbox with a name similar to `manifests-redhat-operator-index-1646254114` and the path
to that directory should be stored in the `$MANIFESTS` environment variable.

In this directory are two YAML files that need to be applied to your cluster to mirrored operator
catalog. Run:

```
oc apply -f $MANIFESTS/catalogSource.yaml
oc apply -f $MANIFESTS/imageContentSourcePolicy.yaml
```

Verify the resources were created sucessfully:

```
$ oc -n openshift-marketplace get catalogsources
NAME                    DISPLAY               TYPE   PUBLISHER   AGE
redhat-operator-index                         grpc               17m

$ oc -n openshift-marketplace get pods
NAME                                    READY   STATUS             RESTARTS       AGE
marketplace-operator-5777f8869b-b4ngz   1/1     Running            1 (121m ago)   136m
redhat-operator-index-qvlnl             1/1     Running            0              19m
```

The `redhat-operator-index-*` pod should be running. 

```
$ oc -n openshift-marketplace get packagemanifest
NAME                          CATALOG   AGE
multicluster-engine                     20m
advanced-cluster-management             20m
```

You should see the operators that you pruned into the operator catalog when you mirrored it.

  
## Configure the ACM Operator

### Inspect the ACM operator

View the versions of ACM available to install:

```
oc -n openshift-marketplace get packagemanifest advanced-cluster-management -o json > acm.json

jq '.status.channels[].name' acm.json
```

You'll see the possible versions to install, such as:

```
"release-2.3"
"release-2.4"
```

Save the ACM version you plan on using:

```
export ACM_CHANNEL=release-2.4
echo export ACM_CHANNEL=$ACM_CHANNEL >> $HOME/.bash_profile
```

### Create the ACM subscription

```
cat <<EOF >> acm-subscription.yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: acm-operator-subscription
spec:
  sourceNamespace: openshift-marketplace
  source: redhat-operator-index
  channel: $ACM_CHANNEL
  installPlanApproval: Automatic
  name: advanced-cluster-management
EOF

oc apply -f acm-subscription.yaml
```

At this point, ACM is available to install.


## Install ACM

### Create the ACM namespace

The default namespace is `open-cluster-management`. Run:

```
export ACM_NAMESPACE=open-cluster-management
echo export ACM_NAMESPACE=$ACM_NAMESPACE >> $HOME/.bash_profile

oc create namespace $ACM_NAMESPACE
oc project $ACM_NAMESPACE
```

### Create the pull secret in the ACM namespace

This pull secret needs to have credentials for the mirror registry. We'll use the
`auth.json` file we created earlier when we created the mirror registry.

```
export ACM_PULL_SECRET=open-cluster-management-pull-secret
echo export ACM_PULL_SECRET=$ACM_PULL_SECRET >> $HOME/.bash_profile

oc create secret generic -n $ACM_NAMESPACE $ACM_PULL_SECRET \
    --from-file=.dockerconfigjson=auth.json \
    --type=kubernetes.io/dockerconfigjson
```

### Create an operator group

```
cat <<EOF >> operatorgroup.yaml
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: open-cluster-management-operator-group
spec:
  targetNamespaces:
  - $ACM_NAMESPACE
EOF

oc apply -f operatorgroup.yaml
```

### Create the MultiClusterHub custom resource

```
cat <<EOF >> multiclusterhub.yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  name: multiclusterhub
  namespace: $ACM_NAMESPACE
spec:
  imagePullSecret: $ACM_PULL_SECRET
EOF

oc apply -f multiclusterhub.yaml
```

### Watch ACM installation

Run:

```
$ oc get -n $ACM_NAMESPACE mch -o=jsonpath='{.items[0].status.phase}' ; echo
oc get -n open-cluster-management mch -o=jsonpath='{.items[0].status.phase}' ; echo
Installing
```

You can also run:

```
watch -n 15 -g oc get --kubeconfig=/home/ec2-user/ocp-install/auth/kubeconfig \
    -n $ACM_NAMESPACE mch -o=jsonpath='{.items[0].status.phase}'
```

This will check the status every 15 seconds and exit when the output changes from `Installing`.

When the phase is `Running`, ACM installation will be complete. It may 10 minutes or more for the 
MultiClusterHub resource to reach the Running phase.

### Verify the route has been added:

```
$ oc get -n $ACM_NAMESPACE routes
NAME                 HOST/PORT                                                   PATH   SERVICES             PORT    TERMINATION          WILDCARD
multicloud-console   multicloud-console.apps.my-dc1.dev01.red-chesterfield.com          management-ingress   https   reencrypt/Redirect   None
```
