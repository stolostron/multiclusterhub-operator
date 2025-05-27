#!/usr/bin/env python3
# Copyright (c) 2024 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project
# Assumes: Python 3.6+

import argparse
import coloredlogs
import os
import logging
import subprocess
import shutil
import sys
import time

from pathlib import Path
from git import Repo

# Configure logging with coloredlogs
coloredlogs.install(level='DEBUG')  # Set the logging level as needed

TMP_DIR = Path(__file__).resolve().parent / "tmp/dev-tools"
SCRIPTS_DIR = TMP_DIR / "scripts"
DEST_DIR = Path(__file__).resolve().parent
SUPPORTED_OPERATIONS = {
    "copy-charts": {
        "script": "bundle-generation/move-charts.py",
        "args": "--destination pkg/templates/",
        "help": "Copy existing Helm charts to a target directory",
    },
    "lint-bundles": {
        "script": "bundle-generation/bundles-to-charts.py",
        "args": "--lint --destination pkg/templates/",
        "help": "Perform linting for operator bundles",
    },
    "onboard-new-components": {
        "script": "release/onboard-new-components.py",
        "args": "",
        "help": "Onboard new component configurations by adding them to the operator's resource definitions",
    },
    "refresh-image-aliases": {
        "script": "release/refresh-image-aliases.py",
        "args": "--repo {pipeline_repo} --branch {pipeline_branch}",
        "help": "Refresh image alias mappings for the specified repository and branch, updating them for new versions",
    },
    "update-charts": {
        "script": "bundle-generation/generate-charts.py",
        "args": "--destination pkg/templates/",
        "help": "Convert standard Helm charts to customized versions for your specific use case",
    },
    "update-charts-from-bundles": {
        "script": "bundle-generation/bundles-to-charts.py",
        "args": "--destination pkg/templates/",
        "help": "Generate Helm charts from OpenShift Operator Lifecycle Manager (OLM) bundles",
    },
    "update-commits": {
        "script": "bundle-generation/generate-sha-commits.py",
        "args": "--repo {pipeline_repo} --branch {pipeline_branch}",
        "help": "Synchronize commit SHA values in operator bundles with the latest repository changes",
    },
}

def clone_repository(git_url, repo_path, branch):
    """Clones a Git repository to a specific path.

    Args:
        git_url (_type_): _description_
        repo_path (_type_): _description_
        branch (_type_): _description_
    """
    if os.path.exists(repo_path):
        logging.warning(f"Repository path: {repo_path} already exists. Removing existing directory.")
        shutil.rmtree(repo_path)

    logging.info(f"Cloning repository: {git_url} (branch={branch}) to {repo_path}")
    try:
        repository = Repo.clone_from(git_url, repo_path)
        repository.git.checkout(branch)
        logging.info(f"Git repository: {git_url} successfully cloned.")

    except Exception as e:
        logging.error(f"Failed to clone repository: {git_url} (branch={branch}): {e}")
        raise

def copy_scripts(script_dependencies):
    """Copies necessary scripts from the temporary directory.

    Args:
        script_dependencies (_type_): _description_
    """
    for dependency in script_dependencies:
        src = SCRIPTS_DIR / dependency
        dest = DEST_DIR / Path(dependency).name

        if not src.exists():
            logging.error(f"Required script or directory {src} not found.")
            sys.exit(1)

        if src.is_dir():
            shutil.copytree(src, dest, dirs_exist_ok=True)
            logging.debug(f"Copied directory {src} to {dest}")

        else:
            shutil.copy(src, dest)
            logging.debug(f"Copied file {src} to {dest}")

def cleanup_scripts(script_dependencies):
    """Cleans up copied scripts from the destination directory.

    Args:
        script_dependencies (_type_): _description_
    """
    for script in script_dependencies:
        # If the script is in a subdirectory like 'utils/', construct the path properly
        dest = DEST_DIR / script

        # Check if the destination path is a directory or a file
        if dest.exists():
            if dest.is_dir():
                # If the destination is a directory (e.g., utils/), remove it and its contents
                shutil.rmtree(dest)
                logging.debug(f"Removed directory {dest}")
            else:
                # If the destination is a file, unlink it
                dest.unlink(missing_ok=True)
                logging.debug(f"Removed file {dest}")

def prepare_and_execute(operation, operation_data, args):
    """Prepares and executes the operation based on the provided operation data.

    Args:
        operation (_type_): _description_
        operation_data (_type_): _description_
        args (_type_): _description_
    """
    logging.info(f"Executing operator: {operation}")

    script = Path(operation_data["script"])
    script_dependencies = [script, "utils"]
    copy_scripts(script_dependencies)

    operations_args = operation_data.get("args", "").format(
        pipeline_repo=args.pipeline_repo,
        pipeline_branch=args.pipeline_branch
    ) if "args" in operation_data else ""
    
    if args.component:
        operations_args += " --component {}".format(args.component)

    if args.config:
        operations_args += " --config {}".format(args.config)

    # Execute the script
    execute_script(script, operations_args)

    # Clean up the copied scripts
    cleanup_scripts(script_dependencies)

def execute_script(script, args):
    """Executes a Python script with arguments.

    Args:
        script (_type_): _description_
        args (_type_): _description_
    """
    script_path = DEST_DIR / Path(script).name

    if not script_path.exists():
        logging.error(f"Script {script_path} not found.")
        sys.exit(1)

    command = ["python3", str(script_path)] + args.split()
    try:
        subprocess.run(command, check=True)
    except subprocess.CalledProcessError as e:
        logging.error(f"Script {script} failed with exit code {e.returncode}")
        sys.exit(e.returncode)
    finally:
        script_path.unlink(missing_ok=True)  # Clean up after execution

def main(args):
    """_summary_

    Args:
        args (_type_): _description_
    """
    logging.basicConfig(level=logging.INFO)

    start_time = time.time()  # Record start time
    logging.info("🔄 Initiating the generate-shell script for operator bundle management and updates.")

    # Clone the installer-dev-tools repository
    git_url = f"https://github.com/{args.org}/{args.repo}.git"
    clone_repository(git_url, TMP_DIR, args.branch)

    for operation, operation_data in SUPPORTED_OPERATIONS.items():
        if getattr(args, operation.replace('-', '_'), False):
            prepare_and_execute(operation, operation_data, args)
            break

    end_time = time.time() # Record the end time and log the duration of the script execution
    logging.info(f"Script execution took {end_time - start_time:.2f} seconds.")

if __name__ == "__main__":
    # Set up argument parsing for command-line execution
    parser = argparse.ArgumentParser()

    # Define command-line arguments and their help descriptions
    for operation, operation_data in SUPPORTED_OPERATIONS.items():
        parser.add_argument(
            f"--{operation}",
            action="store_true",
            help=operation_data["help"],
        )

    # Command-Line Arguments
    parser.add_argument("--org", help="GitHub Org name")
    parser.add_argument("--repo", help="Github Repo name")
    parser.add_argument("--branch", help="Github Repo Branch name")
    parser.add_argument("--component", help="Target Component")
    parser.add_argument("--config", help="Target Config file")
    parser.add_argument("--pipeline-repo", help="Pipeline Repository name")
    parser.add_argument("--pipeline-branch", help="Pipeline Repository Branch name")

    # Set default values for unspecified arguments
    parser.set_defaults(bundle=False, commit=False, lint=False)

    # Parse command-line arguments and call the main function
    args = parser.parse_args()
    main(args)
