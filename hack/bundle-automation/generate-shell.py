#!/usr/bin/env python3
# Copyright (c) 2024 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project
# Assumes: Python 3.6+

import argparse
import coloredlogs
import os
import logging
import shutil
import time

from git import Repo, exc

# Configure logging with coloredlogs
coloredlogs.install(level='DEBUG')  # Set the logging level as needed

def clone_repository(repo_url, repo_path, branch='main'):
    if os.path.exists(repo_path):
        shutil.rmtree(repo_path)
    logging.info(f"Cloning repository from {repo_url} to {repo_path}...")

    repository = Repo.clone_from(repo_url, repo_path)
    repository.git.checkout(branch)
    logging.info("Repository cloned successfully.")

def prepare_operation(script_dir, operation_script, operation_args):
    shutil.copy(os.path.join(os.path.dirname(os.path.realpath(__file__)), f"{script_dir}/{operation_script}"), os.path.join(os.path.dirname(os.path.realpath(__file__)), operation_script))
    os.system("python3 " + os.path.dirname(os.path.realpath(__file__)) +  f"/{operation_script} {operation_args}")
    os.remove(os.path.join(os.path.dirname(os.path.realpath(__file__)), f"{operation_script}"))

def main(args):
    logging.basicConfig(level=logging.INFO)
    start_time = time.time()  # Record start time

    repo_path = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp/dev-tools") # Destination path for cloned repository.
    clone_repository("https://github.com/dislbenn/installer-dev-tools.git", repo_path)
    script_dir = "tmp/dev-tools/bundle-generation"

    if args.lint_bundles:
        logging.info("Preparing to perform linting for bundles...")
        operation_script = "generate-bundles.py"
        operation_args = "--lint --destination pkg/templates/"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("Bundles linted successfully.")

    elif args.update_bundles:
        logging.info("Preparing to update bundles...")
        operation_script = "generate-bundles.py"
        operation_args = "--destination pkg/templates/"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("Bundles updated successfully.")

    elif args.update_commits:
        logging.info("Preparing to update commit SHAs...")
        operation_script = "generate-sha-commits.py"
        operation_args = f"--repo {args.repo} --branch {args.branch}"

        prepare_operation(script_dir, operation_script, operation_args)
        logging.info("Commit SHAs updated successfully.")

    else:
        logging.warning("No operation specified.")

    end_time = time.time()  # Record end time
    logging.info(f"Script execution took {end_time - start_time:.2f} seconds.")  # Log duration

if __name__ == "__main__":
    parser = argparse.ArgumentParser()

    parser.add_argument("--lint-bundles", action="store_true", help="Perform linting for operator bundles")
    parser.add_argument("--update-bundles", action="store_true", help="Regenerate operator bundles")
    parser.add_argument("--update-commits", action="store_true", help="Regenerate operator bundles with commit SHA")

    parser.add_argument("--repo", help="Repository name")
    parser.add_argument("--branch", default='main', help="Branch name")
    parser.set_defaults(bundle=False, commit=False, lint=False)

    args = parser.parse_args()
    main(args)
