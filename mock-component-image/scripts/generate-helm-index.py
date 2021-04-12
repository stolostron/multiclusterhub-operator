import shutil
import os

# all the env vars
_git_repo_base_dir = os.getcwd() # base repo directory
_mch_repo_dir=os.path.join(_git_repo_base_dir, "multiclusterhub")
_mch_repo_charts_dir=os.path.join(_mch_repo_dir, "charts")

# clean up old charts if they exist
if os.path.isdir(_mch_repo_charts_dir):
    print(_mch_repo_charts_dir)
    os.system('helm repo index --url http://multiclusterhub-repo.open-cluster-management.svc.cluster.local:3000/charts {}'.format(_mch_repo_charts_dir))
else:
    raise Exception ("No Helm Chart Repo!")