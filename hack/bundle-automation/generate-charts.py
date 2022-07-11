#!/usr/bin/env python3
# Copyright (c) 2021 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project
# Assumes: Python 3.6+

import argparse
import os
import shutil
import yaml
import logging
import subprocess
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

def updateServiceAccount(yamlContent):
    yamlContent['metadata'].pop('namespace')

def updateClusterRoleBinding(yamlContent):
    subjectsList = yamlContent['subjects']
    for sub in subjectsList:
        sub['namespace'] = '{{ .Values.global.namespace }}'

# Copy chart-templates to a new helmchart directory
def updateResources(outputDir, repo, chart):
    logging.info(" Updating resources!")
    # Create main folder
    always_or_toggle = chart['always-or-toggle']
    templateDir = os.path.join(outputDir, "charts", always_or_toggle, chart['name'], "templates")
    print(templateDir)
    for tempFile in os.listdir(templateDir):
        filePath = os.path.join(templateDir, tempFile)
        with open(filePath, 'r') as f:
            yamlContent = yaml.safe_load(f)
        kind = yamlContent["kind"]
        if kind == "ServiceAccount":
            logging.info(" Updating ServiceAccount!")
            updateServiceAccount(yamlContent)
        elif kind == "ClusterRoleBinding":
            logging.info(" Updating ClusterRoleBinding!")
            updateClusterRoleBinding(yamlContent)
        else:
            logging.info(" No updates for kind %s at this step.", kind)
            continue
        with open(filePath, 'w') as f:
            yaml.dump(yamlContent, f, width=float("inf"))



# Copy chart-templates to a new helmchart directory
def copyHelmChart(destinationChartPath, repo, chart):
    chartName = chart['name']
    logging.info("Copying templates into new '%s' chart directory ...", chartName)
    # Create main folder
    chartPath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, chart["chart-path"])
    if os.path.exists(destinationChartPath):
        shutil.rmtree(destinationChartPath)
    
    # Copy Chart.yaml, values.yaml, and templates dir

    destinationTemplateDir = os.path.join(destinationChartPath, "templates")
    os.makedirs(destinationTemplateDir)

    specificValues = os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-values", chart['name'], "values.yaml")
    if os.path.exists(specificValues):
        shutil.copyfile(specificValues, os.path.join(chartPath, "values.yaml"))
    helmTemplateOutput = subprocess.getoutput(['helm template '+ chartPath])
    yamlList = helmTemplateOutput.split('---')
    for outputContent in yamlList:
        yamlContent = yaml.safe_load(outputContent)
        if yamlContent is None:
            continue
        newFileName = yamlContent['kind'].lower() + '.yaml'
        newFilePath= os.path.join(destinationTemplateDir, newFileName)
        a_file = open(newFilePath, "w")
        a_file.writelines(outputContent)
        a_file.close()

    chartYamlPath = os.path.join(chartPath, "Chart.yaml")
    if not os.path.exists(chartYamlPath):
        logging.info("No Chart.yaml for chart: ", chartName)
        return
    shutil.copyfile(chartYamlPath, os.path.join(destinationChartPath, "Chart.yaml"))

    shutil.copyfile(os.path.join(chartPath, "values.yaml"), os.path.join(destinationChartPath, "values.yaml"))
    # Copying template values.yaml instead of values.yaml from chart
    shutil.copyfile(os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates", "values.yaml"), os.path.join(destinationChartPath, "values.yaml"))

    logging.info("Chart copied.\n")

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
            yaml.dump(deploy, f, width=float("inf"))

    for imageKey in imageKeys:
        values['global']['imageOverrides'][imageKey] = ""
    with open(valuesYaml, 'w') as f:
        yaml.dump(values, f, width=float("inf"))
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
            args = container['args']
            refreshed_args = []
            for arg in args:
                if "--agent-image-name" not in arg:
                    refreshed_args.append(arg)
                else:
                    refreshed_args.append("--agent-image-name="+"{{ .Values.global.imageOverrides." + image_key + " }}")
            container['args'] = refreshed_args
        with open(deployment, 'w') as f:
            yaml.dump(deploy, f, width=float("inf"))

    del  values['global']['imageOverrides']['imageOverride']
    for imageKey in imageKeys:
        values['global']['imageOverrides'][imageKey] = "" # set to temp to debug
    with open(valuesYaml, 'w') as f:
        yaml.dump(values, f, width=float("inf"))
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

        if 'replicas:' in line.strip():
            lines[i] = """  replicas: {{ .Values.hubconfig.replicaCount }}
"""
        a_file = open(deployment, "w")
        a_file.writelines(lines)
        a_file.close()
    logging.info("Added Helm flow control for NodeSelector and Proxy Overrides.\n")

def addPullSecretOverride(deployment):
    deploy = open(deployment, "r")
    lines = deploy.readlines()
    for i, line in enumerate(lines):
        if line.strip() == "env:" or line.strip() == "env: {}":
            logging.info("Adding image pull secret environment variable to managed-serviceaccount deployment")
            lines[i] = """        env:
{{- if .Values.global.pullSecret }}
        - name: AGENT_IMAGE_PULL_SECRET
          value: {{ .Values.global.pullSecret }}
{{- end }}
"""
        a_file = open(deployment, "w")
        a_file.writelines(lines)
        a_file.close()

# updateDeployments adds standard configuration to the deployments (antiaffinity, security policies, and tolerations)
def updateDeployments(chartName, helmChart, exclusions, inclusions):
    logging.info("Updating deployments with antiaffinity, security policies, and tolerations ...")
    deploySpecYaml = os.path.join(os.path.dirname(os.path.realpath(__file__)), "chart-templates/templates/deploymentspec.yaml")
    with open(deploySpecYaml, 'r') as f:
        deploySpec = yaml.safe_load(f)
    
    deployments = findTemplatesOfType(helmChart, 'Deployment')
    for deployment in deployments:
        with open(deployment, 'r') as f:
            deploy = yaml.safe_load(f)
        deploy['metadata'].pop('namespace')
        deploy['spec']['replicas'] = "{{ .Values.hubconfig.replicaCount }}"
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
            container['securityContext']['runAsNonRoot'] = True
            if 'readOnlyRootFilesystem' not in exclusions:
                container['securityContext']['readOnlyRootFilesystem'] = True
        
        with open(deployment, 'w') as f:
            yaml.dump(deploy, f, width=float("inf"))
        logging.info("Deployments updated with antiaffinity, security policies, and tolerations successfully. \n")

        injectHelmFlowControl(deployment)
        if 'pullSecretOverride' in inclusions:
            addPullSecretOverride(deployment)

# updateRBAC adds standard configuration to the RBAC resources (clusterroles, roles, clusterrolebindings, and rolebindings)
def updateRBAC(helmChart, chartName):
    logging.info("Updating clusterroles, roles, clusterrolebindings, and rolebindings ...")
    clusterroles = findTemplatesOfType(helmChart, 'ClusterRole')
    roles = findTemplatesOfType(helmChart, 'Role')
    clusterrolebindings = findTemplatesOfType(helmChart, 'ClusterRoleBinding')
    rolebindings = findTemplatesOfType(helmChart, 'RoleBinding')

    for rbacFile in clusterroles + roles + clusterrolebindings + rolebindings:
        with open(rbacFile, 'r') as f:
            rbac = yaml.safe_load(f)
        rbac['metadata']['name'] = "{{ .Values.org }}:{{ .Chart.Name }}:" + chartName
        if rbac['kind'] in ['RoleBinding', 'ClusterRoleBinding']:
            rbac['roleRef']['name'] = "{{ .Values.org }}:{{ .Chart.Name }}:" + chartName
        with open(rbacFile, 'w') as f:
            yaml.dump(rbac, f, width=float("inf"))
    logging.info("Clusterroles, roles, clusterrolebindings, and rolebindings updated. \n")


def injectRequirements(helmChart, chartName, imageKeyMapping, exclusions, inclusions):
    logging.info("Updating Helm chart '%s' with onboarding requirements ...", helmChart)
    fixImageReferences(helmChart, imageKeyMapping)
    fixEnvVarImageReferences(helmChart, imageKeyMapping)
    updateRBAC(helmChart, chartName)
    updateDeployments(chartName, helmChart, exclusions, inclusions)
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

def addCRDs(repo, chart, outputDir):
    if not 'chart-path' in chart:
        logging.critical("Could not validate chart path in given chart: " + chart)
        exit(1) 

    chartPath = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp", repo, chart["chart-path"])
    if not os.path.exists(chartPath):
        logging.critical("Could not validate chartPath at given path: " + chartPath)
        exit(1)
    
    crdPath = os.path.join(chartPath, "crds")
    if not os.path.exists(crdPath):
        logging.info("No CRDs for repo: ", repo)
        return

    destinationPath = os.path.join(outputDir, chart['name'], "crds")
    if os.path.exists(destinationPath): # If path exists, remove and re-clone
        shutil.rmtree(destinationPath)
    os.makedirs(destinationPath)
    for filename in os.listdir(crdPath):
        if not filename.endswith(".yaml"): 
            continue
        filepath = os.path.join(crdPath, filename)
        with open(filepath, 'r') as f:
            resourceFile = yaml.safe_load(f)

        if resourceFile["kind"] == "CustomResourceDefinition":
            shutil.copyfile(filepath, os.path.join(destinationPath, filename))

def chartConfigAcceptable(chart):
    helmChart = chart["name"]
    if helmChart == "":
        logging.critical("Unable to generate helm chart without a name.")
        return False
    return True

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
    configYaml = os.path.join(os.path.dirname(os.path.realpath(__file__)),"charts-config.yaml")
    with open(configYaml, 'r') as f:
        config = yaml.safe_load(f)

    # Loop through each repo in the config.yaml
    for repo in config:
        logging.info("Cloning: %s", repo["repo_name"])
        repo_path = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp/" + repo["repo_name"]) # Path to clone repo to
        if os.path.exists(repo_path): # If path exists, remove and re-clone
            shutil.rmtree(repo_path)
        repository = Repo.clone_from(repo["github_ref"], repo_path) # Clone repo to above path
        if 'branch' in repo:
            repository.git.checkout(repo['branch']) # If a branch is specified, checkout that branch
        
        # Loop through each operator in the repo identified by the config
        for chart in repo["charts"]:
            if not chartConfigAcceptable(chart):
                logging.critical("Unable to generate helm chart without configuration requirements.")
                exit(1)

            logging.info("Helm Chartifying -  %s!\n", chart["name"])

            logging.info("Adding CRDs -  %s!\n", chart["name"])
            # Copy over all CRDs to the destination directory
            addCRDs(repo["repo_name"], chart, destination)

            logging.info("Creating helm chart: '%s' ...", chart["name"])

            always_or_toggle = chart['always-or-toggle']
            destinationChartPath = os.path.join(destination, "charts", always_or_toggle, chart['name'])

            # Template Helm Chart Directory from 'chart-templates'
            logging.info("Templating helm chart '%s' ...", chart["name"])
            copyHelmChart(destinationChartPath, repo["repo_name"], chart)

            updateResources(destination, repo["repo_name"], chart)

            if not skipOverrides:
                logging.info("Adding Overrides (set --skipOverrides=true to skip) ...")
                exclusions = chart["exclusions"] if "exclusions" in chart else []
                inclusions = chart["inclusions"] if "inclusions" in chart else []
                injectRequirements(destinationChartPath, chart["name"], chart["imageMappings"], exclusions, inclusions)
                logging.info("Overrides added. \n")

if __name__ == "__main__":
   main()
