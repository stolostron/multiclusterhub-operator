#!/usr/bin/env python3
# Copyright (c) 2021 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project
# Assumes: Python 3.6+

import argparse
import os
import shutil
import yaml
import array
import logging
import sys
from git import Repo, exc

from validate_csv import *

# Parse an image reference, return dict containing image reference information
def parse_image_ref(image_ref):
   # Image ref:  [registry-and-ns/]repository-name[:tag][@digest]
   parsed_ref = dict()

   remaining_ref = image_ref
   at_pos = remaining_ref.rfind("@")
   if at_pos > 0:
      parsed_ref["digest"] = remaining_ref[at_pos+1:]
      remaining_ref = remaining_ref[0:at_pos]
   else:
      parsed_ref["digest"] = None
   colon_pos = remaining_ref.rfind(":")
   if colon_pos > 0:
      parsed_ref["tag"] = remaining_ref[colon_pos+1:]
      remaining_ref = remaining_ref[0:colon_pos]
   else:
      parsed_ref["tag"] = None
   slash_pos = remaining_ref.rfind("/")
   if slash_pos > 0:
      parsed_ref["repository"] = remaining_ref[slash_pos+1:]
      rgy_and_ns = remaining_ref[0:slash_pos]
   else:
      parsed_ref["repository"] = remaining_ref
      rgy_and_ns = "localhost"
   parsed_ref["registry_and_namespace"] = rgy_and_ns

   rgy, ns = split_at(rgy_and_ns, "/", favor_right=False)
   if not ns:
      ns = ""

   parsed_ref["registry"] = rgy
   parsed_ref["namespace"] = ns

   slash_pos = image_ref.rfind("/")
   if slash_pos > 0:
      repo_and_suffix = image_ref[slash_pos+1:]
   else:
      repo_and_suffix = image_ref
   parsed_ref["repository_and_suffix"]  = repo_and_suffix

   return parsed_ref

# Copy chart-templates to a new helmchart directory
def templateHelmChart(outputDir, helmChart):
    logging.info("Copying templates into new '%s' chart directory ...", helmChart)
    # Create main folder
    if os.path.exists(os.path.join(outputDir, "charts", "toggle",  helmChart)):
        shutil.rmtree(os.path.join(outputDir, "charts","toggle", helmChart))

    # Create Chart.yaml, values.yaml, and templates dir
    os.makedirs(os.path.join(outputDir, "charts", "toggle",  helmChart, "templates"))
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates", "Chart.yaml"), os.path.join(outputDir, "charts",  "toggle", helmChart, "Chart.yaml"))
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates", "values.yaml"), os.path.join(outputDir, "charts", "toggle", helmChart, "values.yaml"))
    logging.info("Templates copied.\n")

# Fill in the chart.yaml template with information from the CSV
def fillChartYaml(helmChart, name, csvPath):
    logging.info("Updating '%s' Chart.yaml file ...", helmChart)
    chartYml = os.path.join(helmChart, "Chart.yaml")

    # Read Chart.yaml
    with open(chartYml, 'r') as f:
        chart = yaml.safe_load(f)

    # logging.info("%s", csvPath)
    # Read CSV    
    with open(csvPath, 'r') as f:
        csv = yaml.safe_load(f)

    logging.info("Chart Name: %s", helmChart)
    

    # Write to Chart.yaml
    chart['name'] = name
    
    if 'metadata' in csv:
        if 'annotations' in csv ["metadata"]:
            if 'description' in csv["metadata"]["annotations"]:
                logging.info("Description: %s", csv["metadata"]["annotations"]["description"])
                chart['description'] = csv["metadata"]["annotations"]["description"]
    # chart['version'] = csv['metadata']['name'].split(".", 1)[1][1:]
    with open(chartYml, 'w') as f:
        yaml.dump(chart, f)
    logging.info("'%s' Chart.yaml updated successfully.\n", helmChart)

# Copy chart-templates/deployment, update it with CSV deployment information, and add to chart
def addDeployment(helmChart, deployment):
    name = deployment["name"]
    logging.info("Templating deployment '%s.yaml' ...", name)

    deployYaml = os.path.join(helmChart, "templates",  name + ".yaml")
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/deployment.yaml"), deployYaml)

    with open(deployYaml, 'r') as f:
        deploy = yaml.safe_load(f)
        
    deploy['spec'] = deployment['spec']
    if 'spec' in deploy:
        if 'template' in deploy['spec']:
            if 'spec' in deploy['spec']['template']:
                if 'imagePullPolicy' in deploy['spec']['template']['spec']:
                    del deploy['spec']['template']['spec']['imagePullPolicy']
    deploy['metadata']['name'] = name
    with open(deployYaml, 'w') as f:
        yaml.dump(deploy, f)
    logging.info("Deployment '%s.yaml' updated successfully.\n", name)

# Copy chart-templates/clusterrole,clusterrolebinding,serviceaccount.yaml update it with CSV information, and add to chart
def addClusterScopedRBAC(helmChart, rbacMap):
    name = rbacMap["serviceAccountName"]
    # name = "not-default"
    
    logging.info("Setting cluster scoped RBAC ...")
    logging.info("Templating clusterrole '%s-clusterrole.yaml' ...", name)
    
    # Create Clusterrole
    clusterroleYaml = os.path.join(helmChart, "templates",  name + "-clusterrole.yaml")
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/clusterrole.yaml"), clusterroleYaml)
    with open(clusterroleYaml, 'r') as f:
        clusterrole = yaml.safe_load(f)
    # Edit Clusterrole
    clusterrole["rules"] = rbacMap["rules"]
    clusterrole["metadata"]["name"] = name
    # Save Clusterrole
    with open(clusterroleYaml, 'w') as f:
        yaml.dump(clusterrole, f)
    logging.info("Clusterrole '%s-clusterrole.yaml' updated successfully.", name)
    
    logging.info("Templating serviceaccount '%s-serviceaccount.yaml' ...", name)
    # Create Serviceaccount
    serviceAccountYaml = os.path.join(helmChart, "templates",  name + "-serviceaccount.yaml")
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/serviceaccount.yaml"), serviceAccountYaml)
    with open(serviceAccountYaml, 'r') as f:
        serviceAccount = yaml.safe_load(f)
    # Edit Serviceaccount
    serviceAccount["metadata"]["name"] = name
    # Save Serviceaccount
    with open(serviceAccountYaml, 'w') as f:
        yaml.dump(serviceAccount, f)
    logging.info("Serviceaccount '%s-serviceaccount.yaml' updated successfully.", name)

    logging.info("Templating clusterrolebinding '%s-clusterrolebinding.yaml' ...", name)
    # Create Clusterrolebinding
    clusterrolebindingYaml = os.path.join(helmChart, "templates",  name + "-clusterrolebinding.yaml")
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/clusterrolebinding.yaml"), clusterrolebindingYaml)
    with open(clusterrolebindingYaml, 'r') as f:
        clusterrolebinding = yaml.safe_load(f)
    clusterrolebinding['metadata']['name'] = name
    clusterrolebinding['roleRef']['name'] = clusterrole["metadata"]["name"]
    clusterrolebinding['subjects'][0]['name'] = name
    with open(clusterrolebindingYaml, 'w') as f:
        yaml.dump(clusterrolebinding, f)
    logging.info("Clusterrolebinding '%s-clusterrolebinding.yaml' updated successfully.", name)
    logging.info("Cluster scoped RBAC created.\n")

# Copy over role, rolebinding, and serviceaccount templates from chart-templates/templates, update with CSV information, and add to chart
def addNamespaceScopedRBAC(helmChart, rbacMap):
    name = rbacMap["serviceAccountName"]
    # name = "not-default"
    logging.info("Setting namespaced scoped RBAC ...")
    logging.info("Templating role '%s-role.yaml' ...", name)
    # Create role
    roleYaml = os.path.join(helmChart, "templates",  name + "-role.yaml")
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/role.yaml"), roleYaml)
    with open(roleYaml, 'r') as f:
        role = yaml.safe_load(f)
    # Edit role
    role["rules"] = rbacMap["rules"]
    role["metadata"]["name"] = name
    # Save role
    with open(roleYaml, 'w') as f:
        yaml.dump(role, f)
    logging.info("Role '%s-role.yaml' updated successfully.", name)
    
    # Create Serviceaccount
    serviceAccountYaml = os.path.join(helmChart, "templates",  name + "-serviceaccount.yaml")
    if not os.path.isfile(serviceAccountYaml):
        logging.info("Serviceaccount doesnt exist. Templating '%s-serviceaccount.yaml' ...", name)
        shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/serviceaccount.yaml"), serviceAccountYaml)
        with open(serviceAccountYaml, 'r') as f:
            serviceAccount = yaml.safe_load(f)
        # Edit Serviceaccount
        serviceAccount["metadata"]["name"] = name
        # Save Serviceaccount
        with open(serviceAccountYaml, 'w') as f:
            yaml.dump(serviceAccount, f)
        logging.info("Serviceaccount '%s-serviceaccount.yaml' updated successfully.", name)

    logging.info("Templating rolebinding '%s-rolebinding.yaml' ...", name)
    # Create rolebinding
    rolebindingYaml = os.path.join(helmChart, "templates",  name + "-rolebinding.yaml")
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/rolebinding.yaml"), rolebindingYaml)
    with open(rolebindingYaml, 'r') as f:
        rolebinding = yaml.safe_load(f)
    rolebinding['metadata']['name'] = name
    rolebinding['roleRef']['name'] = role["metadata"]["name"] = name
    rolebinding['subjects'][0]['name'] = name
    with open(rolebindingYaml, 'w') as f:
        yaml.dump(rolebinding, f)
    logging.info("Rolebinding '%s-rolebinding.yaml' updated successfully.", name)
    logging.info("Namespace scoped RBAC created.\n")

# Adds resources identified in the CSV to the helmchart
def addResources(helmChart, csvPath):
    logging.info("Reading CSV '%s'\n", csvPath)

    # Read CSV    
    with open(csvPath, 'r') as f:
        csv = yaml.safe_load(f)
    
    logging.info("Checking for deployments, clusterpermissions, and permissions.\n")
    # Check for deployments
    for deployment in csv['spec']['install']['spec']['deployments']:
        addDeployment(helmChart, deployment)
    # Check for clusterroles, clusterrolebindings, and serviceaccounts
    if 'clusterPermissions' in csv['spec']['install']['spec']:
        clusterPermissions = csv['spec']['install']['spec']['clusterPermissions']
        for clusterRole in clusterPermissions:
            addClusterScopedRBAC(helmChart, clusterRole)
    # Check for roles, rolebindings, and serviceaccounts
    if 'permissions' in csv['spec']['install']['spec']:
        permissions = csv['spec']['install']['spec']['permissions']
        for role in permissions:
            addNamespaceScopedRBAC(helmChart, role)
    logging.info("Resources have been successfully added to chart '%s' from CSV '%s'.\n", helmChart, csvPath)
    
    logging.info("Check to see if there are resources in the csv that aren't getting picked up")
    handleAllFiles = False
    # Current list of resources we handle
    listOfResourcesAdded = ["deployments", "clusterPermissions", "permissions", "CustomResourceDefinition"]
    for resource in csv['spec']['install']['spec']:
        if resource not in listOfResourcesAdded:
            logging.error("Found a resource in the csv not being handled called '%s' in '%s'", resource, csvPath)
            handleAllFiles = True

    logging.info("Copying over other resources in the bundle if they exist ...")
    dirPath = os.path.dirname(csvPath)
    logging.info("From directory '%s'", dirPath)
    otherBundleResourceTypes = ["ClusterRole", "ClusterRoleBinding", "Role", "RoleBinding", "Service", "ConfigMap"]
    # list of files we handle currently
    listOfFilesAdded = ["ClusterRole", "ClusterRoleBinding", "Role", 
    "RoleBinding", "Service", "ClusterManagementAddOn", "CustomResourceDefinition", "ClusterServiceVersion", "ConfigMap"]
    for filename in os.listdir(dirPath):
        if filename.endswith(".yaml") or filename.endswith(".yml"):
            filePath = os.path.join(dirPath, filename)
            with open(filePath, 'r') as f:
                fileYml = yaml.safe_load(f)
            if fileYml['kind'] in otherBundleResourceTypes:
                shutil.copyfile(filePath, os.path.join(helmChart, "templates", os.path.basename(filePath)))
            if fileYml['kind'] not in listOfFilesAdded:
                logging.error("Found a file of a resource that is not being handled called '%s' in '%s", fileYml['kind'],dirPath)
                handleAllFiles = True
            continue
        else:
            continue
    if handleAllFiles:
        logging.error("Found a resource in either the manifest or csv we aren't handling")
        sys.exit(1)
# Given a resource Kind, return all filepaths of that resource type in a chart directory
def findTemplatesOfType(helmChart, kind):
    resources = []
    for filename in os.listdir(os.path.join(helmChart, "templates")):
        if filename.endswith(".yaml") or filename.endswith(".yml"):
            filePath = os.path.join(helmChart, "templates", filename)
            with open(filePath, 'r') as f:
                fileYml = yaml.safe_load(f)
            if fileYml['kind'] == kind:
                resources.append(filePath)
            continue
        else:
            continue
    return resources

# For each deployment, identify the image references if any exist in the environment variable fields, insert helm flow control code to reference it, and add image-key to the values.yaml file.
# If the image-key referenced in the deployment does not exist in `imageMappings` in the Config.yaml, this will fail. Images must be explicitly defined
def fixEnvVarImageReferences(helmChart, imageKeyMapping):
    logging.info("Fixing image references in container 'env' section in deployments and values.yaml ...")
    valuesYaml = os.path.join(helmChart, "values.yaml")
    with open(valuesYaml, 'r') as f:
        values = yaml.safe_load(f)
    deployments = findTemplatesOfType(helmChart, 'Deployment')

    imageKeys = []
    for deployment in deployments:
        with open(deployment, 'r') as f:
            deploy = yaml.safe_load(f)
        
        containers = deploy['spec']['template']['spec']['containers']
        for container in containers:
            if 'env' not in container: 
                continue
            
            for env in container['env']:
                image_key = env['name']
                if image_key.endswith('_IMAGE') == False:
                    continue
                image_key = parse_image_ref(env['value'])['repository']
                try:
                    image_key = imageKeyMapping[image_key]
                except KeyError:
                    logging.critical("No image key mapping provided for imageKey: %s" % image_key)
                    exit(1)
                imageKeys.append(image_key)
                env['value'] = "{{ .Values.global.imageOverrides." + image_key + " }}"
        with open(deployment, 'w') as f:
            yaml.dump(deploy, f)

    for imageKey in imageKeys:
        values['global']['imageOverrides'][imageKey] = ""
    with open(valuesYaml, 'w') as f:
        yaml.dump(values, f)
    logging.info("Image container env references in deployments and values.yaml updated successfully.\n")

# For each deployment, identify the image references if any exist in the image field, insert helm flow control code to reference it, and add image-key to the values.yaml file.
# If the image-key referenced in the deployment does not exist in `imageMappings` in the Config.yaml, this will fail. Images must be explicitly defined
def fixImageReferences(helmChart, imageKeyMapping):
    logging.info("Fixing image and pull policy references in deployments and values.yaml ...")
    valuesYaml = os.path.join(helmChart, "values.yaml")
    with open(valuesYaml, 'r') as f:
        values = yaml.safe_load(f)
    
    deployments = findTemplatesOfType(helmChart, 'Deployment')
    imageKeys = []
    temp = "" ## temporarily read image ref
    for deployment in deployments:
        with open(deployment, 'r') as f:
            deploy = yaml.safe_load(f)
        
        containers = deploy['spec']['template']['spec']['containers']
        for container in containers:
            image_key = parse_image_ref(container['image'])["repository"]
            try:
                image_key = imageKeyMapping[image_key]
            except KeyError:
                logging.critical("No image key mapping provided for imageKey: %s" % image_key)
                exit(1)
            imageKeys.append(image_key)
            # temp = container['image'] 
            container['image'] = "{{ .Values.global.imageOverrides." + image_key + " }}"
            container['imagePullPolicy'] = "{{ .Values.global.pullPolicy }}"
        with open(deployment, 'w') as f:
            yaml.dump(deploy, f)

    del  values['global']['imageOverrides']['imageOverride']
    for imageKey in imageKeys:
        values['global']['imageOverrides'][imageKey] = "" # set to temp to debug
    with open(valuesYaml, 'w') as f:
        yaml.dump(values, f)
    logging.info("Image references and pull policy in deployments and values.yaml updated successfully.\n")

# injectHelmFlowControl injects advanced helm flow control which would typically make a .yaml file more difficult to parse. This should be called last.
def injectHelmFlowControl(deployment):
    logging.info("Adding Helm flow control for NodeSelector and Proxy Overrides ...")
    deploy = open(deployment, "r")
    lines = deploy.readlines()
    for i, line in enumerate(lines):
        if line.strip() == "nodeSelector: \'\'":
            lines[i] = """{{- with .Values.hubconfig.nodeSelector }}
      nodeSelector:
{{ toYaml . | indent 8 }}
{{- end }}
"""     
        if line.strip() == "imagePullSecrets: \'\'":
            lines[i] = """{{- if .Values.global.pullSecret }}
      imagePullSecrets:
      - name: {{ .Values.global.pullSecret }}
{{- end }}
"""
        if line.strip() == "tolerations: \'\'":
            lines[i] = """{{- with .Values.hubconfig.tolerations }}
      tolerations:
      {{- range . }}
      - {{ if .Key }} key: {{ .Key }} {{- end }}
        {{ if .Operator }} operator: {{ .Operator }} {{- end }}
        {{ if .Value }} value: {{ .Value }} {{- end }}
        {{ if .Effect }} effect: {{ .Effect }} {{- end }}
        {{ if .TolerationSeconds }} tolerationSeconds: {{ .TolerationSeconds }} {{- end }}
        {{- end }}
{{- end }}
"""


        if line.strip() == "env:" or line.strip() == "env: {}":
            lines[i] = """        env:
{{- if .Values.hubconfig.proxyConfigs }}
        - name: HTTP_PROXY
          value: {{ .Values.hubconfig.proxyConfigs.HTTP_PROXY }}
        - name: HTTPS_PROXY
          value: {{ .Values.hubconfig.proxyConfigs.HTTPS_PROXY }}
        - name: NO_PROXY
          value: {{ .Values.hubconfig.proxyConfigs.NO_PROXY }}
{{- end }}
"""
        a_file = open(deployment, "w")
        a_file.writelines(lines)
        a_file.close()
    logging.info("Added Helm flow control for NodeSelector and Proxy Overrides.\n")

# updateDeployments adds standard configuration to the deployments (antiaffinity, security policies, and tolerations)
def updateDeployments(helmChart, exclusions):
    logging.info("Updating deployments with antiaffinity, security policies, and tolerations ...")
    deploySpecYaml = os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/deploymentspec.yaml")
    with open(deploySpecYaml, 'r') as f:
        deploySpec = yaml.safe_load(f)
    
    deployments = findTemplatesOfType(helmChart, 'Deployment')
    for deployment in deployments:
        with open(deployment, 'r') as f:
            deploy = yaml.safe_load(f)
        affinityList = deploySpec['affinity']['podAntiAffinity']['preferredDuringSchedulingIgnoredDuringExecution']
        for antiaffinity in affinityList:
            antiaffinity['podAffinityTerm']['labelSelector']['matchExpressions'][0]['values'][0] = deploy['metadata']['name']
        deploy['spec']['template']['spec']['affinity'] = deploySpec['affinity']
        deploy['spec']['template']['spec']['tolerations'] = ''
        deploy['spec']['template']['spec']['hostNetwork'] = False
        deploy['spec']['template']['spec']['hostPID'] = False
        deploy['spec']['template']['spec']['hostIPC'] = False
        if 'securityContext' not in deploy['spec']['template']['spec']:
            deploy['spec']['template']['spec']['securityContext'] = {}
        deploy['spec']['template']['spec']['securityContext']['runAsNonRoot'] = True
        deploy['spec']['template']['metadata']['labels']['ocm-antiaffinity-selector'] = deploy['metadata']['name']
        deploy['spec']['template']['spec']['nodeSelector'] = ""
        deploy['spec']['template']['spec']['imagePullSecrets'] = ''

        containers = deploy['spec']['template']['spec']['containers']
        for container in containers:
            if 'securityContext' not in container: 
                container['securityContext'] = {}
            if 'env' not in container: 
                container['env'] = {}
            container['securityContext']['allowPrivilegeEscalation'] = False
            container['securityContext']['capabilities'] = {}
            container['securityContext']['capabilities']['drop'] = ['ALL']
            container['securityContext']['privileged'] = False
            if 'readOnlyRootFilesystem' not in exclusions:
                container['securityContext']['readOnlyRootFilesystem'] = True
        
        with open(deployment, 'w') as f:
            yaml.dump(deploy, f)
        logging.info("Deployments updated with antiaffinity, security policies, and tolerations successfully. \n")

        injectHelmFlowControl(deployment)

# updateRBAC adds standard configuration to the RBAC resources (clusterroles, roles, clusterrolebindings, and rolebindings)
def updateRBAC(helmChart):
    logging.info("Updating clusterroles, roles, clusterrolebindings, and rolebindings ...")
    clusterroles = findTemplatesOfType(helmChart, 'ClusterRole')
    roles = findTemplatesOfType(helmChart, 'Role')
    clusterrolebindings = findTemplatesOfType(helmChart, 'ClusterRoleBinding')
    rolebindings = findTemplatesOfType(helmChart, 'RoleBinding')

    for rbacFile in clusterroles + roles + clusterrolebindings + rolebindings:
        with open(rbacFile, 'r') as f:
            rbac = yaml.safe_load(f)
        rbac['metadata']['name'] = "{{ .Values.org }}:{{ .Chart.Name }}:" + rbac['metadata']['name']
        if rbac['kind'] in ['RoleBinding', 'ClusterRoleBinding']:
            rbac['roleRef']['name'] = "{{ .Values.org }}:{{ .Chart.Name }}:" + rbac['roleRef']['name']
        with open(rbacFile, 'w') as f:
            yaml.dump(rbac, f)
    logging.info("Clusterroles, roles, clusterrolebindings, and rolebindings updated. \n")


def injectRequirements(helmChart, imageKeyMapping, exclusions):
    logging.info("Updating Helm chart '%s' with onboarding requirements ...", helmChart)
    fixImageReferences(helmChart, imageKeyMapping)
    fixEnvVarImageReferences(helmChart, imageKeyMapping)
    updateRBAC(helmChart)
    updateDeployments(helmChart, exclusions)
    logging.info("Updated Chart '%s' successfully\n", helmChart)

def split_at(the_str, the_delim, favor_right=True):

   split_pos = the_str.find(the_delim)
   if split_pos > 0:
      left_part  = the_str[0:split_pos]
      right_part = the_str[split_pos+1:]
   else:
      if favor_right:
         left_part  = None
         right_part = the_str
      else:
         left_part  = the_str
         right_part = None

   return (left_part, right_part)

def addCMAs(repo, operator, outputDir):
    if 'bundlePath' in operator:
        bundlePath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, operator["bundlePath"])
        if not os.path.exists(bundlePath):
            logging.critical("Could not validate bundlePath at given path: " + operator["bundlePath"])
            exit(1)
    else:
        packageYmlPath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, operator["package-yml"])
        if not os.path.exists(packageYmlPath):
            logging.critical("Could not find package.yaml at given path: " + operator["package-yml"])
            exit(1)

        with open(packageYmlPath, 'r') as f:
            packageYml = yaml.safe_load(f)

        bundlePath = ""
        for channel in packageYml["channels"]:
            if channel["name"] == operator["channel"]:
                version = channel["currentCSV"].split(".", 1)[1][1:]
                bundlePath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, os.path.dirname(operator["package-yml"]), version)
                break

        if bundlePath == "":
            print("Unable to find given channel: " +  operator["channel"] + " in package.yaml: " + operator["package-yml"])
            exit(1)

    for filename in os.listdir(bundlePath):
        if not filename.endswith(".yaml"): 
            continue
        filepath = os.path.join(bundlePath, filename)
        with open(filepath, 'r') as f:
            resourceFile = yaml.safe_load(f)

        if resourceFile["kind"] == "ClusterManagementAddOn":
            logging.info("CMA")
            shutil.copyfile(filepath, os.path.join(outputDir, "charts", "toggle", operator['name'], "templates", filename))

def addCRDs(repo, operator, outputDir):
    if 'bundlePath' in operator:
        bundlePath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, operator["bundlePath"])
        if not os.path.exists(bundlePath):
            logging.critical("Could not validate bundlePath at given path: " + operator["bundlePath"])
            exit(1)
    else:
        packageYmlPath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, operator["package-yml"])
        if not os.path.exists(packageYmlPath):
            logging.critical("Could not find package.yaml at given path: " + operator["package-yml"])
            exit(1)

        with open(packageYmlPath, 'r') as f:
            packageYml = yaml.safe_load(f)

        bundlePath = ""
        for channel in packageYml["channels"]:
            if channel["name"] == operator["channel"]:
                version = channel["currentCSV"].split(".", 1)[1][1:]
                bundlePath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, os.path.dirname(operator["package-yml"]), version)
                break

        if bundlePath == "":
            print("Unable to find given channel: " +  operator["channel"] + " in package.yaml: " + operator["package-yml"])
            exit(1)

    directoryPath = os.path.join(outputDir, "crds", operator['name'])
    if os.path.exists(directoryPath): # If path exists, remove and re-clone
        shutil.rmtree(directoryPath)
    os.makedirs(directoryPath)

    for filename in os.listdir(bundlePath):
        if not filename.endswith(".yaml"): 
            continue
        filepath = os.path.join(bundlePath, filename)
        with open(filepath, 'r') as f:
            resourceFile = yaml.safe_load(f)

        if resourceFile["kind"] == "CustomResourceDefinition":
            shutil.copyfile(filepath, os.path.join(outputDir, "crds", operator['name'], filename))

def getCSVPath(repo, operator):
    if 'bundlePath' in operator:
        bundlePath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, operator["bundlePath"])
        if not os.path.exists(bundlePath):
            logging.critical("Could not validate bundlePath at given path: " + operator["bundlePath"])
            exit(1)
    else:
        packageYmlPath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, operator["package-yml"])
        if not os.path.exists(packageYmlPath):
            logging.critical("Could not find package.yaml at given path: " + operator["package-yml"])
            exit(1)

        with open(packageYmlPath, 'r') as f:
            packageYml = yaml.safe_load(f)

        bundlePath = ""
        for channel in packageYml["channels"]:
            if channel["name"] == operator["channel"]:
                version = channel["currentCSV"].split(".", 1)[1][1:]
                bundlePath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, os.path.dirname(operator["package-yml"]), version)
                break

        if bundlePath == "":
            print("Unable to find given channel: " +  operator["channel"] + " in package.yaml: " + operator["package-yml"])
            exit(1)

    for filename in os.listdir(bundlePath):
        if not filename.endswith(".yaml"): 
            continue
        filepath = os.path.join(bundlePath, filename)
        with open(filepath, 'r') as f:
            resourceFile = yaml.safe_load(f)

        if resourceFile["kind"] == "ClusterServiceVersion":
            return filepath

def main():
    ## Initialize ArgParser
    parser = argparse.ArgumentParser()
    parser.add_argument("--destination", dest="destination", type=str, required=False, help="Destination directory of the created helm chart")
    parser.add_argument("--skipOverrides", dest="skipOverrides", type=bool, help="If true, overrides such as helm flow control will not be applied")
    parser.add_argument("--lint", dest="lint", action='store_true', help="If true, bundles will only be linted to ensure they can be transformed successfully. Default is False.")
    parser.set_defaults(skipOverrides=False)
    parser.set_defaults(lint=False)

    args = parser.parse_args()
    skipOverrides = args.skipOverrides
    destination = args.destination
    lint = args.lint

    if lint == False and not destination:
        logging.critical("Destination directory is required when not linting.")
        exit(1)

    logging.basicConfig(level=logging.DEBUG)

    # Config.yaml holds the configurations for Operator bundle locations to be used
    configYaml = os.path.join(os.path.dirname(os.path.realpath(__file__)),"config.yaml")
    with open(configYaml, 'r') as f:
        config = yaml.safe_load(f)

    # Loop through each repo in the config.yaml
    for repo in config:
        csvPath = ""
        logging.info("Cloning: %s", repo["repo_name"])
        repo_path = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp/" + repo["repo_name"]) # Path to clone repo to
        if os.path.exists(repo_path): # If path exists, remove and re-clone
            shutil.rmtree(repo_path)
        repository = Repo.clone_from(repo["github_ref"], repo_path) # Clone repo to above path
        if 'branch' in repo:
            repository.git.checkout(repo['branch']) # If a branch is specified, checkout that branch
        

        # Loop through each operator in the repo identified by the config
        for operator in repo["operators"]:
            logging.info("Helm Chartifying -  %s!\n", operator["name"])
            # Generate and return path to CSV based on bundlePath or package-yml in config.yaml
            csvPath = getCSVPath(repo["repo_name"], operator)
            if csvPath == "":
                # Validate the bundlePath or package-yml path exists in config.yaml
                print("Unable to find given channel: " +  operator["channel"] + " in package.yaml: " + operator["package-yml"])
                exit(1)

            logging.basicConfig(level=logging.DEBUG)

            # Validate CSV exists
            logging.info("Reading CSV: %s ...",  csvPath)
            if not os.path.isfile(csvPath):
                logging.critical("Unable to find CSV at given path - '" + csvPath + "'.")
                exit(1)

            if lint:
                # Lint the CSV
                errs = validateCSV(csvPath)
                if len(errs) > 0:
                    logging.error("CSV Validation errors detected")
                    for err in errs:
                        logging.error(err)
                    exit(1)
                logging.info("CSV validated successfully!\n")
                continue


            # Copy over all CRDs to the destination directory from the manifest folder
            addCRDs(repo["repo_name"], operator, destination)

            # If name is empty, fail
            helmChart = operator["name"]
            if helmChart == "":
                logging.critical("Unable to generate helm chart without a name.")
                exit(1)
            logging.info("Creating helm chart: '%s' ...", operator["name"])

            # Template Helm Chart Directory from 'chart-templates'
            logging.info("Templating helm chart '%s' ...", operator["name"])
            # Creates a helm chart template
            templateHelmChart(destination, operator["name"])
            
            # Generate the Chart.yaml file based off of the CSV
            helmChart = os.path.join(destination, "charts", "toggle", operator["name"])
            logging.info("Filling Chart.yaml ...")
            fillChartYaml(helmChart, operator["name"],csvPath)

            # Add all basic resources to the helm chart from the CSV
            logging.info("Adding Resources from CSV...")
            addResources(helmChart, csvPath)
            logging.info("Resources have been added from CSV. \n")

            # Copy over all ClusterManagementAddons to the destination directory
            addCMAs(repo["repo_name"], operator, destination)

            if not skipOverrides:
                logging.info("Adding Overrides (set --skipOverrides=true to skip) ...")
                exclusions = operator["exclusions"] if "exclusions" in operator else []
                injectRequirements(helmChart, operator["imageMappings"], exclusions)
                logging.info("Overrides added. \n")
    shutil.rmtree((os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp")), ignore_errors=True)       

if __name__ == "__main__":
   main()