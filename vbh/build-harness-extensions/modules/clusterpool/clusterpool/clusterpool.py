# class representing the entire cluster pool
from glob import glob
from os import getenv
from os.path import basename, dirname, isfile
import os
import pathlib
from time import sleep
import logging
from datetime import datetime
from dateutil.parser import parse

from travispy import TravisPy
from travispy.errors import TravisError

from clusterpool import get_travis_github_token, CLEANUP_TRIGGER, RETURN_TRIGGER
from clusterpool.errors import ClusterDead, ClusterNotFound
from clusterpool.lock import GitRepo
from clusterpool.cluster import Cluster, AVAILABLE_NAME, CREATING_NAME, IN_USE_NAME, TAINTED_NAME

# create logger
module_logger = logging.getLogger('clusterpool')

def to_bool(test):
    return str(test).strip().upper() in ['TRUE', '1', 'YES']


class Clusterpool(object):
    def __init__(self, repo, platform, desired_count=0):
        """[summary]

        Arguments:
            repo {string} -- File path indicating where the cluster pool repo is cloned
        """
        self.repo = repo
        self.platform = platform
        self.desired_count = desired_count
        self.git_repo = GitRepo(self.repo, branch=platform)
        self._reserved_cluster = None
        self.allow_failed_create = to_bool(getenv('CLUSTERPOOL_ALLOW_FAILED_CREATE', False))
        self.creation_retries = int(getenv('CLUSTERPOOL_CREATION_RETRIES_ALLOWED', 1))
        self.logger = logging.getLogger('clusterpool.Clusterpool')

    @property
    def reserved_cluster(self):
        return self._reserved_cluster

    @property
    def initialized_flag(self):
        return '{}/initialized-status.md'.format(self.repo)

    @property
    def initialized(self):
        try:
            with open(self.initialized_flag, 'r') as f:
                return to_bool(f.read())
        except FileNotFoundError:
            return False

    @initialized.setter
    def initialized(self, val):
        with self.git_repo as r:
            pathlib.Path(dirname(self.initialized_flag)).mkdir(parents=True, exist_ok=True)
            with open(self.initialized_flag, 'w') as f:
                f.write(str(val))
            r.index.add([self.initialized_flag])
            # if we are setting initialized to True, then skip CI
            suffix = ''
            if to_bool(val):
                suffix = ' [skip ci]'
            r.index.commit('change {} initialization to {}{}'.format(self.platform, val, suffix))

    def checkout_cluster(self):
        module_logger.debug('clusterpool.checkout_cluster() entry.')
        if self._reserved_cluster is None:
            self._reserved_cluster = Cluster(self.git_repo, self.platform)
            self._reserved_cluster.checkout()
        return self._reserved_cluster

    def return_cluster(self):
        module_logger.debug('clusterpool.return_cluster() entry.')
        if self._reserved_cluster is not None:
            self._reserved_cluster.put_back()
            returned_cluster = self._reserved_cluster
            self._reserved_cluster = None
            return returned_cluster

    def destroy_cluster(self, name, force, debug):
        module_logger.debug('clusterpool.destroy_cluster() entry.  cluster={} force={} debug={}'.format(name, force, debug))

        self._reserved_cluster = Cluster(self.git_repo, self.platform, name)
        module_logger.debug('self._reserved_cluster.name: {}'.format(self._reserved_cluster.name))
        self._reserved_cluster.checkout()

        self._reserved_cluster.put_back(override=force)
        returned_cluster = self._reserved_cluster
        self._reserved_cluster = None
        return returned_cluster

    def upgrade(self):
        # For each cluster, run the destroy route: try to check it out, and immediately put it back.
        # That ensures that 1) no one is currently using it, and 2) when it gets reincarnated, it
        # will have fresh install.
        module_logger.debug('clusterpool.upgrade() entry.')
        self.git_repo.origin.pull()
        null_cluster = Cluster(self.git_repo, self.platform, '**')
        cluster_dir = null_cluster.cluster_dir

        module_logger.info('Checking for available jobs to upgrade.')
        for available in glob('{}/{}'.format(cluster_dir, AVAILABLE_NAME)):
            name = basename(dirname(available))
            module_logger.debug('clusterpool.upgrade() upgrading soon-to-be-destroyed cluster {}.'.format(name))
            self.destroy_cluster(name, True, False)
                
    def get_cluster(self, name):
        module_logger.debug('clusterpool.get_cluster() entry.')
        # get a cluster without checking it out
        return Cluster(self.git_repo, self.platform, name)

    def check(self, commit_message=None, should_clean=True):
        self.git_repo.origin.pull()
        null_cluster = Cluster(self.git_repo, self.platform, '**')
        cluster_dir = null_cluster.cluster_dir
        clusters_available = glob('{}/{}'.format(cluster_dir, AVAILABLE_NAME))
        clusters_building = glob('{}/{}'.format(cluster_dir, CREATING_NAME))
        clusters_in_use =  glob('{}/{}'.format(cluster_dir, IN_USE_NAME))
        clusters_tainted =  glob('{}/{}_*'.format(cluster_dir, TAINTED_NAME))

        module_logger.info('Cluster status for platform {}:'.format(self.platform))
        if to_bool(should_clean):
            module_logger.info('\tdesired: {}'.format(self.desired_count))
        module_logger.info('\tavailable: {}\n{}'.format(len(clusters_available), clusters_available))
        module_logger.info('\tbuilding: {}\n{}'.format(len(clusters_building), clusters_building))
        module_logger.info('\tin use: {}\n{}'.format(len(clusters_in_use), clusters_in_use))
        module_logger.info('\ttainted: {}\n{}'.format(len(clusters_tainted), clusters_tainted))

        if to_bool(should_clean) == False:
            self._check_cluster_known_state(AVAILABLE_NAME, 'available', should_clean)
            self._check_cluster_known_state(IN_USE_NAME, 'in-use', should_clean)
            self._check_cluster_known_state(CREATING_NAME, 'creating', should_clean)
        else:
            trigger = ''
            if commit_message is not None:
                trigger = commit_message.split(' ')[0]
    
            if trigger == RETURN_TRIGGER:
                # we are returning a cluster, so cleanup if the platform matches
                platform = commit_message.split(' ')[1]
                name = commit_message.split(' ')[2]
                if platform == self.platform:
                    cluster = Cluster(self.git_repo, self.platform, name)
                    cluster.delete()
            elif trigger == CLEANUP_TRIGGER:
                module_logger.info('We have detected that cleanup is needed!')
                self.cleanup()
                module_logger.info('Cleanup finished. Pausing 120s before checking to see if more clusters need to be added.')
                sleep(120)
                self.initialized = False
                self.check()
            elif len(clusters_available) + len(clusters_building) < self.desired_count:
                module_logger.info('Cluster needed!')
                cluster = Cluster(self.git_repo, self.platform, tries=self.creation_retries, allow_failed_create=self.allow_failed_create)
                cluster.generate(self.initialized)
            else:
                module_logger.info('No clusters needed.')
                # we have enough clusters, so make sure that we are initialized.
                if not self.initialized:
                    module_logger.info('Ensuring that the repo is now set to initialized.')
                    self.initialized = True

    def cleanup(self):
        # for each cluster, ensure that a connection can be made and delete otherwise
        # a new cluster creation should be fired off for each one deleted (i.e. each in its own push or uninitializing)
        null_cluster = Cluster(self.git_repo, self.platform, '**')
        cluster_dir = null_cluster.cluster_dir
        checked_dirs = []
        checked_dirs.extend(self._check_cluster_known_state(AVAILABLE_NAME, 'available', True))
        checked_dirs.extend(self._check_cluster_known_state(IN_USE_NAME, 'in-use', True))
                
        module_logger.info('Checking for failed jobs')
        for creating in glob('{}/{}'.format(cluster_dir, CREATING_NAME)):
            name = basename(dirname(creating))
            checked_dirs.append(name)
            self._check_cluster_health(name, True)

        module_logger.info('Cleaning up clusters without a current state')
        for unknown in glob('{}'.format(cluster_dir[0:-1])):
            name = basename(unknown)
            if name not in checked_dirs:
                cluster = Cluster(self.git_repo, self.platform, name)
                cluster.put_back(override=True)
                
    def _check_cluster_health(self, cluster_name, should_clean):
        module_logger.info('Checking health of cluster {}.'.format(cluster_name))
        cluster = Cluster(self.git_repo, self.platform, cluster_name)
        if not cluster_name.isdigit():
            try:
                cluster.is_alive()
                module_logger.debug('Cluster {} is alive.'.format(cluster_name))
            except ClusterDead:
                module_logger.debug('Cluster {} is dead.'.format(cluster_name))
                if to_bool(should_clean) == True:
                    module_logger.info('Cluster dead; cleaning up.')
                    cluster.put_back(override=True)
            except ClusterNotFound:
                # This cluster might have been cleaned up already, so skip
                module_logger.debug('Cluster {} is not found.'.format(cluster_name))
                module_logger.info('Cluster not found; it is probably already cleaned up.')
            try:
                t = TravisPy.github_auth(
                    get_travis_github_token(), uri='https://travis.ibm.com/api')
            except TravisError:
                raise Exception(
                    'Authentication to Travis failed! You need to provide a GitHub token at $(GITHUB_TOKEN) with the scopes read:org, repo, user:email, write:repo_hook')
            job = t.job(cluster_name[9:])
            delta_time = datetime.utcnow()-datetime.strptime(job.finished_at, '%Y-%m-%dT%H:%M:%SZ')
            module_logger.info('Cluster age: {} hours'.format(round(delta_time.total_seconds()/3600,2)))

            in_use_file = '{}/{}'.format(cluster.cluster_dir, IN_USE_NAME)
            if os.path.isfile(in_use_file):
                with open(in_use_file, 'r') as f:
                    self.identifying_info = f.read()
                    module_logger.info(self.identifying_info)
            module_logger.info('')

        else:
            try:
                t = TravisPy.github_auth(
                    get_travis_github_token(), uri='https://travis.ibm.com/api')
            except TravisError:
                raise Exception(
                    'Authentication to Travis failed! You need to provide a GitHub token at $(TRAVIS_TOKEN) with the scopes read:org, repo, user:email, write:repo_hook')
        
            job = t.job(cluster_name)
            module_logger.info('Job status: {}'.format(job.color))
            if job.color != job.YELLOW and to_bool(should_clean) == True:
                module_logger.info('Job not in progress; cleaning up.')
                cluster.put_back(file_rm=CREATING_NAME, override=True)
            module_logger.info('')

    def _check_cluster_known_state(self, state_file, state_name, should_clean):
        null_cluster = Cluster(self.git_repo, self.platform, '**')
        cluster_dir = null_cluster.cluster_dir
        checked_clusters = []
        module_logger.info('')
        module_logger.info('Checking clusters marked as {}'.format(state_name))
        module_logger.info('')
        for avail in glob('{}/{}'.format(cluster_dir, state_file)):
            name = basename(dirname(avail))
            checked_clusters.append(name)
            self._check_cluster_health(name, should_clean)
        return checked_clusters
