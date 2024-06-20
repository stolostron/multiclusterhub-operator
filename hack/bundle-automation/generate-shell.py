from git import Repo, exc
import os
import shutil


repo_path = os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp/dev-tools") # Path to clone repo to
if os.path.exists(repo_path): # If path exists, remove and re-clone
    shutil.rmtree(repo_path)
repository = Repo.clone_from("https://github.com/stolostron/installer-dev-tools.git", repo_path) # Clone repo to above path
repository.git.checkout('main') # If a branch is specified, checkout that branch
shutil.copy(os.path.join(os.path.dirname(os.path.realpath(__file__)), "tmp/dev-tools/bundle-generation/bundles-to-charts.py"), os.path.join(os.path.dirname(os.path.realpath(__file__)), "bundles-to-charts.py"))
os.system("python3 " + os.path.dirname(os.path.realpath(__file__)) +  "/bundles-to-charts.py --destination pkg/templates/")
os.remove(os.path.join(os.path.dirname(os.path.realpath(__file__)), "bundles-to-charts.py"))