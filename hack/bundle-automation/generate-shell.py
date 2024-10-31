#!/usr/bin/env python3
# Copyright (c) 2024 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project
# Assumes: Python 3.6+

import argparse
import coloredlogs
import os
import logging
import shutil
import sys
import time

from git import Repo, exc

# Configure logging with coloredlogs
coloredlogs.install(level='DEBUG')  # Set the logging level as needed

def clone_repository(git_url, repo_path, branch):
    if os.path.exists(repo_path):
        logging.warning(f"Repository path: {repo_path} already exists. Removing existing directory.")
        shutil.rmtree(repo_path)

    logging.info(f"Cloning Git repository: {git_url} (branch={branch}) to {repo_path}")
    try:
        repository = Repo.clone_from(git_url, repo_path)
        repository.git.checkout(branch)
        logging.info(f"Git repository: {git_url} successfully cloned.")

    except Exception as e:
        logging.error(f"Failed to clone Git repository: {git_url} (branch={branch}): {e}.")
        raise

def prepare_operation(script_dir, operation_script, operation_args):
    shutil.copy(os.path.join(os.path.dirname(os.path.realpath(__file__)), f"{script_dir}/{operation_script}"), os.path.join(os.path.dirname(os.path.realpath(__file__)), operation_script))
    exit_code = os.system("python3 " + os.path.dirname(os.path.realpath(__file__)) +  f"/{operation_script} {operation_args}")
    if exit_code != 0:
        sys.exit(1)

    os.remove(os.path.join(os.path.dirname(os.path.realpath(__file__)), f"{operation_script}"))

def main(args):
    logging.basicConfig(level=logging.INFO)

    start_time = time.time()  # Record start time
    logging.info("🔄 Initiating the generate-shell script for operator bundle management and updates.")

    # Extract org, repo, branch, pipeline_repo, and pipeline_branch from command-line arguments
    # Use the specified org and branch or the defaults ('stolostron', 'installer-dev-tools', 'main', 'pipeline', '2.10-integration')
    org = args.org
    repo = args.repo
    branch = args.branch
    pipeline_repo = args.pipeline_repo
    pipeline_branch = args.pipeline_branch

    # Define the destination path for the cloned repository
    repo_path = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp/dev-tools")

    # Clone the repository using the specified git_url, destination path, and branch
    git_url = f"https://github.com/{org}/{repo}.git"
    clone_repository(git_url, repo_path, branch)

    # Define the directory containing the bundle generation scripts
    script_dir = "tmp/dev-tools/bundle-generation"

    # Check which operation is requested based on command-line arguments
    if args.lint_bundles:
        logging.info("Starting linting for bundles...")
        operation_script = "bundles-to-charts.py"
        operation_args = "--lint --destination pkg/templates/"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("✔️ Bundles linted successfully.")

    elif args.update_charts_from_bundles:
        logging.info("Updating operator charts from bundles...")
        operation_script = "bundles-to-charts.py"
        operation_args = "--destination pkg/templates/"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("✔️ Bundles updated successfully.")

    elif args.update_charts:
        logging.info("Updating operator charts...")
        operation_script = "generate-charts.py"
        operation_args = "--destination pkg/templates/"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("✔️ Bundles updated successfully.")

    elif args.copy_charts:
        logging.info("Copying charts...")
        operation_script = "move-charts.py"
        operation_args = "--destination pkg/templates/"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("✔️ Bundles updated successfully.")

    elif args.update_commits:
        logging.info("Updating commit SHAs...")
        operation_script = "generate-sha-commits.py"
        operation_args = f"--repo {pipeline_repo} --branch {pipeline_branch}"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("✔️ Commit SHAs updated successfully.")

    else:
        logging.warning("⚠️ No operation specified.")

    # Record the end time and log the duration of the script execution
    end_time = time.time()
    logging.info(f"Script execution took {end_time - start_time:.2f} seconds.")

if __name__ == "__main__":
    # Set up argument parsing for command-line execution
    parser = argparse.ArgumentParser()

    # Define command-line arguments and their help descriptions
    parser.add_argument("--lint-bundles", action="store_true", help="Perform linting for operator bundles")
    parser.add_argument("--update-charts-from-bundles", action="store_true", help="Regenerate operator charts from bundles")
    parser.add_argument("--update-commits", action="store_true", help="Regenerate operator bundles with commit SHA")
    parser.add_argument("--update-charts", action="store_true", help="Regenerate operator charts")
    parser.add_argument("--copy-charts", action="store_true", help="Copy operator charts")

    parser.add_argument("--org", help="GitHub Org name")
    parser.add_argument("--repo", help="Github Repo name")
    parser.add_argument("--branch", help="Github Repo Branch name")
    parser.add_argument("--pipeline-repo", help="Pipeline Repository name")
    parser.add_argument("--pipeline-branch", help="Pipeline Repository Branch name")

    # Set default values for unspecified arguments
    parser.set_defaults(bundle=False, commit=False, lint=False)

    # Parse command-line arguments and call the main function
    args = parser.parse_args()
    main(args)
