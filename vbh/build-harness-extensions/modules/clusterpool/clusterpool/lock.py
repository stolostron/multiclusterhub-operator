import logging
from random import uniform
from shutil import rmtree
from time import sleep

from git import Repo, GitError
from gitdb.exc import BadName

from clusterpool import identifying_info

LOCKFILE_NAME = 'lockfile.md'

# see: https://stackoverflow.com/questions/3774328/implementing-use-of-with-object-as-f-in-custom-class-in-python

# create logger
module_logger = logging.getLogger('clusterpool')

class GitRepo(object):
    def __init__(self, repository, filename=LOCKFILE_NAME, max_iterations=100, branch='master'):
        self._repository_dir = repository
        self.repository = Repo(repository)
        self.unique = identifying_info()
        self.lockfile = filename
        self.origin = self.repository.remote('origin')
        self.lockfile_path = '{}/{}'.format(repository, filename)
        self.max_iterations = max_iterations
        self._sha_ref = self.repository.head.reference.commit.hexsha
        self.branch = branch

    @property
    def branch(self):
        return self._branch
    
    @branch.setter
    def branch(self, target):
        # ensure that the repo has the latest remote heads
        self.origin.fetch()
        #self.repository.delete_head('temp_branch')
        self.repository.git.reset('--hard', 'HEAD^')
        #self.repository.reset(working_tree=True)
        self.repository.git.clean('-fd')
        self.repository.git.pull()
        try:
            self.repository.git.checkout('--track', 'origin/{}'.format(target))
            self.repository.git.push()
        except GitError:
            self.repository.git.checkout(target)
            # self.repository.git.branch('--set-upstream-to=origin/{}'.format(target), target)
            # self.repository.git.push()
        # switch to a temporary branch
        # temp_head = self.repository.create_head('temp_branch', 'HEAD')
        # self.repository.head.reference = temp_head
        # delete the target branch locally if it exists and recreate it
        # try:
        #     self.repository.delete_head(self.repository.heads[target], force=True)
        # except IndexError:
        #     # If we have an error, it does not exist
        #     pass
        #branch = self.repository.create_head(target)
        # ensure that the new branch is pushed to origin and update it
        # try:
        #     self.repository.git.push('origin', target)
        # except GitError:
        #     # If we have an error, then the branch is already in origin.
        #     pass
        self.origin = self.repository.remote('origin')
        # set the head reference to the remote branch
        # branch.set_tracking_branch(self.origin.refs[target])
        # self.repository.head.reference = branch
        self.repository.head.reset(working_tree=True)
        self.repository.git.clean('-fd')
        self.origin.pull()
        self._branch = target
        # self.repository.delete_head(temp_head)

    @property
    def git_dir(self):
        return self._repository_dir

    def __enter__(self):
        """Enter a lock by reserving the lockfile with your identifying info
        If you have any local changes before entering this lock, they will be wiped out.
        """
        if self.repository is None:
            # this info has to be reset after deserialization
            self.repository = Repo(self.git_dir)
            self.origin = self.repository.remote('origin')
            self.branch = self._branch
        print('trying to get the cluster pool lock')
        self.unique = identifying_info()
        self._safe_pull()
        self._sha_ref = self.repository.head.reference.commit.hexsha
        iterations = 0
        while True:
            iterations += 1
            lockfile_contents = self._read_from_lockfile(sleep_max=2)
            module_logger.debug('lockfile contents as read from git: {}'.format(lockfile_contents))
 
            if iterations >= self.max_iterations:
                raise(Exception('Lock file could not be claimed in {} iterations; failing'.format(
                    self.max_iterations)))
            # if we have the lock, we have entered
            if lockfile_contents == self.unique:
                print('got the lock')
                break
            # if someone else has the lock, continue to loop
            elif lockfile_contents != 'released':
                module_logger.info('Waiting for lockfile to be released; contents: {}'.format(lockfile_contents))
                continue
            else:
                module_logger.debug('preparing to reserve lock with my info: {}'.format(self.unique))
            self._write_to_lockfile(self.unique, 'reserve lock')
        # Ensure that we have pulled the latest before returning the repo
        self.origin.pull()
        return self.repository

    def __exit__(self, type, value, traceback):
        """Exit a lock

        Arguments:
            type {[type]} -- [description]
            value {[type]} -- [description]
            traceback {[type]} -- [description]
        """
        # ensure that all committed changes are pushed
        self.origin.push()
        # return the lockfile
        lockfile_contents = self._read_from_lockfile()
        if lockfile_contents != self.unique:
            raise(Exception('You did not have the lock; cannot release!'))
        self._write_to_lockfile('released', 'release lock')
        iterations = 0
        while True:
            iterations += 1
            lockfile_contents = self._read_from_lockfile(sleep_max=2)
            module_logger.debug('lockfile contents to write: {}'.format(lockfile_contents))

            if iterations >= self.max_iterations:
                raise(Exception('Lock file could not be claimed in {} iterations; failing'.format(
                    self.max_iterations)))
            if lockfile_contents != self.unique:
                print('released the lock!')
                break
            print('Failed to release the lock, trying again')
            self._write_to_lockfile('released', 'release lock')

    def _write_to_lockfile(self, contents, commit_msg):
        # Do not pull before we write to the lockfile. We want an error if it changes under us
        with open(self.lockfile_path, 'w') as lf:
            lf.write(contents)
        self.repository.index.add([self.lockfile_path])
        self.repository.index.commit('{} [skip ci]'.format(commit_msg))
        self._safe_push()

    def _read_from_lockfile(self, sleep_min=.8, sleep_max=8):
        if sleep_max < sleep_min:
            # swap it!
            sleep_min, sleep_max = sleep_max, sleep_min
        sleep(uniform(sleep_min, sleep_max))
        self._safe_pull()
        with open(self.lockfile_path, 'r') as lf:
            lockfile_contents = lf.read().strip()
        return lockfile_contents

    def _safe_pull(self):
        self.branch = self._branch
        try:
            self.origin.pull()
        except (GitError, BadName) as e:
            module_logger.error('safe pull failed: {}'.format(e))
            if self._sha_ref is not None:
                print('Trying a hard reset to a known state')
                self.repository.head.reset(
                    commit=self._sha_ref, working_tree=True)
                self.origin.pull()
            else:
                raise e

    def _safe_push(self):
        try:
            self.origin.push()
        except (GitError) as e:
            module_logger.error('safe push failed: {}'.format(e))
            self.branch = self._branch

    def __getstate__(self):
        out_dict = self.__dict__
        del out_dict['repository']
        del out_dict['origin']
        return out_dict

    def __setstate__(self, in_dict):
        in_dict['repository'] = None
        in_dict['origin'] = None
        self.__dict__ = in_dict
