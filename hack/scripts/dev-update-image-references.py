#!/usr/local/bin/python3
# Copyright Contributors to the Open Cluster Management project
import os
from git import Repo
import glob
import json
import yaml
import argparse
import shutil

def getLatestManifest():
    pipelineDir = os.path.join(os.getcwd(), "bin/pipeline")
    if os.path.exists(pipelineDir):
        shutil.rmtree(pipelineDir)
    repo = Repo.clone_from("https://github.com/stolostron/pipeline.git", pipelineDir)
    repo.git.checkout("2.10-integration")
    manifests = glob.glob('bin/pipeline/snapshots/manifest-*.json')
    manifests.sort()
    return manifests[-1]

def getOperandImagesDictionary(latestManifest):
    manifest = open(latestManifest)
    imageRefs = json.load(manifest)
    operandImages = []
    for imageRef in imageRefs:
        imageKey = 'OPERAND_IMAGE_' + imageRef['image-key'].upper()
        imageKey = imageKey.replace('-', '_')
        image = "{imageRemote}/{imageName}@{imageDigest}".format(imageRemote=imageRef['image-remote'], imageName=imageRef['image-name'], imageDigest=imageRef['image-digest'])
        operandImages.append({'name': imageKey, 'value': image})
    return operandImages

def updateContainerWithEnvVars(containerYaml, operandImages):
    if 'env' in containerYaml:
        preexistingVars = containerYaml['env']
        preexistingVars = [x for x in preexistingVars if not x['name'].startswith('OPERAND_IMAGE')]
        operandImages.extend(preexistingVars)
    containerYaml['env'] = operandImages

def addImageRefsToDeploymentYaml(deployYaml, operandImages):
    with open(deployYaml) as f:
        managerDocs = yaml.load_all(f, yaml.SafeLoader)
        yamlArr = []
        for doc in managerDocs:
            if doc['kind'] == 'Deployment' and doc['metadata']['name'] == 'multiclusterhub-operator':
                for container in doc['spec']['template']['spec']['containers']:
                    updateContainerWithEnvVars(container, operandImages)
            yamlArr.append(doc)
        
        with open(deployYaml, 'w') as file:
            yaml.dump_all(yamlArr, file, Dumper=yaml.SafeDumper)

def setImageReferencesInLocalEnvironment(operandImages):
    envVarsFile = "env-vars.txt"
    try:
        os.remove(envVarsFile)
    except OSError:
        pass
    with open(envVarsFile,"a+") as f:
        for imageRef in operandImages:
            f.write('export ' + imageRef['name'] + "=" + imageRef['value'] + "\n")
    

def main():

    parser = argparse.ArgumentParser(description='Process local env vars')
    parser.add_argument('--local', dest='local', type=bool, nargs='?',
                        const=True, default=False,
                    help='Set image references as local environment variables')

    args = parser.parse_args()
    latestManifest = getLatestManifest()
    operandImages = getOperandImagesDictionary(latestManifest)
    if args.local == True:
        print("Setting locally")
        setImageReferencesInLocalEnvironment(operandImages)
        print("env-vars.txt created. Run 'source env-vars.txt' to define the environment variables in your local environment")
    else:
        addImageRefsToDeploymentYaml('config/manager/manager.yaml', operandImages)
        print("Image references added in deployment")


if __name__ == "__main__":
    main()
