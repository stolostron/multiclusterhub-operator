# How to refresh hive artifacts


## Make sure you have the proper version of hive repo to compare against
- Get the latest code.
```
cd $GOPATH/src/github.com/openshift
git clone git@github.com:openshift/hive.git
cd hive
git remote update origin --prune
git checkout ocm-4.4.0
```
- Make sure the branch has the latest changes
```
git pull origin ocm-4.4.0
```

## Make sure you have the proper multicloudhub-operator to make changes to
- Get the latest code.
```
cd $GOPATH/src/github.com/open-cluster-management
git clone git@github.com:open-cluster-management/multicloudhub-operator.git
cd multicloudhub-operator
git checkout master
```
- Make sure the branch has the latest changes
```
git pull origin master
```
- Create your branch where the changes will be made.
  - Look at https://quay.io/repository/rhibmcollab/hive?tab=tags to see the latest available version.  Copy the tag name and use that in your branch name
```
git checkout -b <username>-hive-<tag name>
```


## Merge Tool
Use a merge tool to compare what is in hive vs multicloudhub-operator.
**NOTE**: I used Xcode FileMerge to visually merge, customize for your preferred diff tool.  In FileMerge, I check the box **Exclude: Identical** so that I only see files with differeneces.
```
export MERGE_UTIL=/Applications/Xcode.app/Contents/Applications/FileMerge.app/Contents/MacOS/FileMerge
```

## Merge CRD
CRD's are fairly easy because the filenames match
```
export LEFT_PATH=~/go/src/github.com/openshift/hive/config/crds
export RIGHT_PATH=~/go/src/github.com/open-cluster-management/multicloudhub-operator/templates/multiclusterhub/base/hive/crds
$MERGE_UTIL -left $LEFT_PATH  -right $RIGHT_PATH
```

## Merge RBAC
RBAC is a bit harder because the filenames were not kept when brought over.  But here is the current list.  Check to see if there are any new RBAC files that need to be moved over.
```
export LEFT_PATH=~/go/src/github.com/openshift/hive/config
export RIGHT_PATH=~/go/src/github.com/open-cluster-management/multicloudhub-operator/templates/multiclusterhub/base/hive
```
- front end
```
$MERGE_UTIL -left $LEFT_PATH/rbac/hive_frontend_role_binding.yaml  -right $RIGHT_PATH/rbac/hive_frontend_role_binding.yaml
$MERGE_UTIL -left $LEFT_PATH/rbac/hive_frontend_role.yaml  -right $RIGHT_PATH/rbac/hive_frontend_role.yaml
$MERGE_UTIL -left $LEFT_PATH/rbac/hive_frontend_serviceaccount.yaml  -right $RIGHT_PATH/rbac/hive_frontend_serviceaccount.yaml
```

- admission
**NOTE** Not sure - ask devan

```
$MERGE_UTIL -left $LEFT_PATH/hiveadmission/hiveadmission_rbac_role.yaml  -right $RIGHT_PATH/rbac/hiveadmission-role.yaml
$MERGE_UTIL -left $LEFT_PATH/hiveadmission/hiveadmission_rbac_role_binding.yaml  -right $RIGHT_PATH/rbac/hiveadmission-rolebinding.yaml
```
- leave annotation
```
$MERGE_UTIL -left $LEFT_PATH/hiveadmission/service-account.yaml  -right $RIGHT_PATH/rbac/hiveadmission-sa.yaml
```
- controllers role
```
$MERGE_UTIL -left $LEFT_PATH/rbac/hive_controllers_role.yaml  -right $RIGHT_PATH/rbac/hivecontrollers-role.yaml
$MERGE_UTIL -left $LEFT_PATH/rbac/hive_controllers_role_binding.yaml  -right $RIGHT_PATH/rbac/hivecontrollers-rolebinding.yaml
```
keep the following diff in our copy:
```
annotations:
  update-namespace: "false"
```
and this in subjects.default
```
   namespace: hive
```
- operator
```
#-- not sure?
$MERGE_UTIL -left $LEFT_PATH/operator/operator_role.yaml  -right $RIGHT_PATH/rbac/hiveoperator-role.yaml
#-- skip - keep our diffs
$MERGE_UTIL -left $LEFT_PATH/operator/operator_role_binding.yaml  -right $RIGHT_PATH/rbac/hiveoperator-rolebinding.yaml
$MERGE_UTIL -left $LEFT_PATH/operator/operator_deployment.yaml  -right $RIGHT_PATH/rbac/hiveoperator-sa.yaml
```

## Merge Other Config yamls
```
export LEFT_PATH=~/go/src/github.com/openshift/hive/config
export RIGHT_PATH=~/go/src/github.com/open-cluster-management/multicloudhub-operator/templates/multiclusterhub/base/hive

# done
$MERGE_UTIL -left $LEFT_PATH/operator/operator_role.yaml  -right $RIGHT_PATH/rbac/hiveoperator-role.yaml

# skip - keep our diffs
$MERGE_UTIL -left $LEFT_PATH/operator/operator_role_binding.yaml  -right $RIGHT_PATH/rbac/hiveoperator-rolebinding.yaml
$MERGE_UTIL -left $LEFT_PATH/hiveadmission/hiveadmission_rbac_role.yaml  -right $RIGHT_PATH/rbac/hiveadmission-role.yaml
$MERGE_UTIL -left $LEFT_PATH/hiveadmission/hiveadmission_rbac_role_binding.yaml  -right $RIGHT_PATH/rbac/hiveadmission-rolebinding.yaml
$MERGE_UTIL -left $LEFT_PATH/hiveadmission/service-account.yaml  -right $RIGHT_PATH/rbac/hiveadmission-sa.yaml
```

- kustomization

Ensure kustomization file has correct file references for new files




## Update the following files with the new quay hive image tag (search for the old tag and replace):
  https://github.com/open-cluster-management/multicloudhub-operator/blob/master/templates/multicloudhub/base/hive/hive-catalogsource.yaml
   https://github.com/open-cluster-management/multicloudhub-operator/blob/master/templates/multicloudhub/base/hive/hive-operator.yaml

## Build hive pieces so we can get CSV file
We will need a RedHat VM to build this.  I used Amazon Web Services AWS and provisioned a RHEL  

### Install tooling on RHEL VM
SSH to the RHEL VM.  Use the following commands to setup the required tooling
```
sudo bash
yum install -y wget git make

export PATH=$PATH:/usr/local/bin
mkdir /data
cd /data
wget https://dl.google.com/go/go1.13.6.linux-amd64.tar.gz
tar -C /usr/local -xvf go1.13.6.linux-amd64.tar.gz

cd /data
#NOTE: Customize this to download oc from your OpenShift (or setup however you want)
wget https://downloads-openshift-console.apps.eminent-oryx.dev02.red-chesterfield.com/amd64/linux/oc.tar --no-check-certificate
tar -xvf oc.tar
cp oc /usr/local/bin
#NOTE: You may need to `oc login` first
oc version


cd /data
yum install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-$(rpm -E '%{rhel}').noarch.rpm
yum install -y https://centos7.iuscommunity.org/ius-release.rpm
#yum install -y python36u python36u-pip
yum install -y python3 python3-pip
rm /usr/bin/python
ln -s /usr/bin/python3.6 /usr/bin/python
python -V

cd /data
wget https://pypi.python.org/packages/source/s/setuptools/setuptools-7.0.tar.gz --no-check-certificate
tar xzf setuptools-7.0.tar.gz
cd setuptools-7.0
python setup.py install

#cd /data
#wget https://bootstrap.pypa.io/get-pip.py
#python get-pip.py
#pip --version

cd /data
wget https://github.com/mikefarah/yq/releases/download/3.2.1/yq_linux_amd64 -O yq
chmod +x yq
cp yq /usr/local/bin
yq --version


#pip install --upgrade --force-reinstall pip==9.0.3
#pip install yq

yum install -y podman-docker
yum install -y buildah

cd /data
wget https://downloads-openshift-console.apps.eminent-oryx.dev02.red-chesterfield.com/amd64/linux/oc.tar --no-check-certificate
tar -vf oc.tar
cp -f oc /usr/local/bin
oc login https://api.eminent-oryx.dev02.red-chesterfield.com:6443 -u ocpadmin -p best-bashful-bears-balance-berries --insecure-skip-tls-verify=true


mkdir /data/go
cd /data/go
mkdir -p /data/go/src/github.com/openshift
cd /data/go/src/github.com/openshift
git clone https://github.com/openshift/hive.git
cd hive
```

### Patch hive file

Edit file `hack/generate-operator-bundle.py`, around line 34, change from:
```
-full_version = "%s.%s-sha%s" % (VERSION_BASE, git_num_commits, git_hash)
```
to:
```
+full_version = "%s.%s-%s" % (VERSION_BASE, git_num_commits, git_hash)
```

### Run hive script to build CSV
Update the `DEPLOY_IMG` for the https://quay.io/repository/rhibmcollab/hive?tab=tags image you are going to upgrade to
```
#NOTE: Update this for the new image
#export DEPLOY_IMG="quay.io/rhibmcollab/hive:2020-03-23-2154ceae"
export DEPLOY_IMG="quay.io/rhibmcollab/hive:2020-03-24-2ea0bcc0"

#NOTE: You can use your shortname at the end
export REGISTRY_IMG="quay.io/rhibmcollab/multiclusterhub-operator:cahl4"

#NOTE:
hack/olm-registry-deploy.sh
```
**Ensure STEP 1,2 and 3 complete.**  STEP 4 and 5 do not need to run.
The CSV file will be in the bundle directory.  The filename will contain the `commit id` of the new image we want to use.  For example, `bundle/0.1.1774-2154ceae/hive-operator.v0.1.1774-2154ceae.clusterserviceversion.yaml`.

### Compare CSV file to our hive-operator.yaml
- Make sure of the following metadata:
  - annotations:
    - `containerImage` is correct for version we are upgrading to
    - `createdAt` is not needed
    - Don't merge `support`
  - `namespace` make sure it remains **hive**
```
$MERGE_UTIL -left ./TEMP-hive-operator.v0.1.1774-2154ceae.clusterserviceversion.yaml  -right $RIGHT_PATH/hive-operator.yaml
```

## Testing
### Install
- Go back to the multicloudhub-operator repo
```
cd multicloudhub-operator
```
- Export a few environment variables
```
export DOCKER_PASS=<quay.io encrypted password>
export DOCKER_USER=<quay.io username>
export GITHUB_TOKEN=<personal access token>
export GITHUB_USER=<username>
export NAMESPACE=cahl
export VERSION=cahl
```
- In deploy/kustomization.yaml, make the following changes.
```
-namespace: default
+namespace: cahl
-  newTag: latest
+  newTag: cahl
```
- Create new namespace for testing in
```
oc create ns cahl
```
- Run Install
```
./common/scripts/tests/install.sh
```
- The script will pause and wait for you to type `done` to continue.  **BEFORE typing `done`, edit the listed file**
- Edit `deploy/crds/operators.open-cluster-management.io_v1alpha1_multiclusterhub_cr.yaml`
(NOTE: This file only exists after ./common/scripts/tests/install.sh runs the first time and
       stops while waiting for user input.)
  - Set `ocpHost` and `imageTagSuffix`.  For example:
  ```
  # imageTagSuffix: "SNAPSHOT-2020-03-12-01-24-50"
  # ocpHost: "eminent-oryx-install.dev02.red-chesterfield.com"
```

### Uninstall
You may need to iterate a few times with your changes, so in that case you will need to uninstall and install
make uninstall
