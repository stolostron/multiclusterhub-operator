from git import Repo, exc
import os
import shutil


repo_path = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp/bundle-gen") # Path to clone repo to
if os.path.exists(repo_path): # If path exists, remove and re-clone
    shutil.rmtree(repo_path)
repository = Repo.clone_from("https://github.com/stolostron/installer-dev-tools.git", repo_path) # Clone repo to above path
repository.git.checkout('main') # If a branch is specified, checkout that branch

os.system("python3 " + repo_path +  "/bundle-generation/generate-bundles.py --destination pkg/templates/ --configLocation MCHconfig.yaml")
shutil.rmtree((os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp")), ignore_errors=True)       