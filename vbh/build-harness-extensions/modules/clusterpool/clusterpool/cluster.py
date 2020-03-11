from contextlib import contextmanager
from glob import glob
import json
import os
from random import choice, randint
import re
import shutil
import logging
from stat import S_IRUSR
from socket import timeout as sock_timeout
from subprocess import run, PIPE, CalledProcessError
from tempfile import TemporaryDirectory

import paramiko

from pathlib import Path
from clusterpool import identifying_info, creating_id, CLEANUP_TRIGGER, RETURN_TRIGGER, INSTANCE_NAME
from clusterpool.errors import ClusterException, ClusterDead, ClusterNotFound, ClusterNotInitialized, NoClustersAvailable

# create logger
module_logger = logging.getLogger('clusterpool')

AVAILABLE_NAME = 'available'
CREATING_NAME = 'creating.md'
TAINTED_NAME = 'taint'
IN_USE_NAME = 'in-use.md'
CLUSTER_DIRECTORY = 'clusters'

DEPLOY_DIR_NAME = '.clusterpool_deploy_dir'

MAKE_LINE = re.compile(r'^make\[\d\]:')


@contextmanager
def with_cd(path):
    old_dir = os.getcwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(old_dir)


def filter_out_make_commands(output, num_lines=None):
    lines = output.decode('utf-8').split('\n')
    filtered = [l for l in lines if not re.match(
        MAKE_LINE, l) and l.strip() != '']
    if num_lines is 0:
        num_lines = None
    if num_lines is not None:
        num_lines *= -1
        filtered = filtered[num_lines:]
    return '\n'.join(filtered).strip()


def touch(filename):
    if not os.path.exists(filename):
        open(filename, 'w').close()


class Cluster(object):
    def __init__(self, git_repo, platform, name=None, tries=1, allow_failed_create=False):
        # Can we check for the presence of a build harness here?
        module_logger.debug('Cluster ctor entry.  name: {}'.format(name))
        self._temp_deploy_dir = None
        self.git_repo = git_repo
        self.platform = platform
        self.name = name
        self.identifying_info = identifying_info()
        self._cluster_claimed = False
        self._ssh_file = None
        self.cluster_directory = CLUSTER_DIRECTORY
        self.max_tries = tries
        self.allow_failed_create = allow_failed_create

    @property
    def cluster_dir(self):
        if self.name is not None:
            return '{}/{}/{}'.format(self.git_repo.git_dir, self.cluster_directory, self.name)

    @property
    def deploy_dir(self):
        if self._temp_deploy_dir is not None:
            return self._temp_deploy_dir
        if self.name is not None:
            return '{}/{}'.format(self.cluster_dir, DEPLOY_DIR_NAME)

    @property
    def private_key_file(self):
        if self._ssh_file is not None:
            return self._ssh_file
        key = '{}/id_rsa'.format(self.git_repo.git_dir)
        # Allow for backwards compatibility -- if we have clusters with keys specific to them
        if self.name is not None and os.path.isfile('{}/id_rsa'.format(self.cluster_dir)):
            key = '{}/id_rsa'.format(self.cluster_dir)
            if not os.path.isfile('{}/id_rsa'.format(self.cluster_dir)):
                key = '{}/id_rsa'.format(self.git_repo.git_dir)
        if not os.path.isfile(key):
            key = None
        else:
            os.chmod(key, S_IRUSR)
        self._ssh_file = key
        return key

    @property
    def tf_variable_file(self):
        if self.name is not None:
            return '{}/.{}.tfvars'.format(self.cluster_dir, self.platform)

    @property
    def modules_json(self):
        return '{}/.terraform/modules/modules.json'.format(self.deploy_dir)

    @property
    def ssh_user(self):
        if 'openshift4-aws' in self.platform:
            return 'ec2-user'
        return self._get_terraform_output('ocp_admin_username', length_of_output=1).strip()

    @property
    def master_node(self):
        return self._get_terraform_output('install_machine_fqhn', length_of_output=1).strip()

    @property
    def icp_web_console(self):
        return self._get_terraform_output('icp-web-console', length_of_output=1).strip()

    def _get_terraform_output(self, output_var, terraform_dir=None, length_of_output=None):
        if terraform_dir is None:
            terraform_dir = self.deploy_dir
        if 'openshift4-aws' in self.platform:
            tfstate_file='{}/aws-ipi/terraform.tfstate'.format(terraform_dir)
        else:
            tfstate_file='{}/terraform.tfstate'.format(terraform_dir)
        with with_cd(self.git_repo.git_dir):
            with TemporaryDirectory() as temp_tfstate_dir:
                temp_tfstate_dir = str(Path(temp_tfstate_dir).resolve())
                module_logger.debug("terraform output requested for variable '{}' from tfstate file {}".format(output_var, tfstate_file))
                if os.path.isfile('{}.enc'.format(tfstate_file)):
                    decrypted_tfstate_file = '{}/terraform.tfstate'.format(temp_tfstate_dir)
                    run_cmd='openssl enc -aes-256-cbc -d -in {}.enc -out {} -k {}'.format(tfstate_file, decrypted_tfstate_file, 'afro-donkey-seldom-waterfall-compute')
                    run(run_cmd, shell=True)
                else:
                    decrypted_tfstate_file = tfstate_file
                run('make -s -S terraform:install TERRAFORM_STATE_FILE={}'.format(decrypted_tfstate_file), shell=True, stdout=PIPE)
                completed_process = run(
                    'make -s -S terraform:output TERRAFORM_OUTPUT_VAR={} TERRAFORM_STATE_FILE={}'.format(output_var, decrypted_tfstate_file), shell=True, stdout=PIPE)
        final_output = filter_out_make_commands(completed_process.stdout, int(length_of_output))
        if completed_process.returncode != 0:
            module_logger.error("terraform output for {} failed: {}".format(output_var,final_output))
            final_output = 'error_state_{}'.format(self.name)
        else:
            module_logger.debug("terraform output for '{}' is: {}".format(output_var,final_output)) 
        return final_output

    def _modify_modules_json(self, index=0, key='Dir', value=None):
        if not os.path.isfile(self.modules_json):
            # We do not need to modify the file if it doesn't exist!
            return
        with open(self.modules_json, 'r') as f:
            modules = json.load(f)
        if key == 'Dir':
            if value is None:
                value = self.deploy_dir
            old_dir = modules['Modules'][index][key]
            value = '{}/.terraform/modules/{}'.format(
                value, os.path.basename(old_dir))
        modules['Modules'][index][key] = value
        with open(self.modules_json, 'w') as f:
            json.dump(modules, f)

    def generate(self, initialized):
        with self.git_repo as r:
            creating_dir = '{}/{}/{}'.format(r.working_dir,
                                             self.cluster_directory, creating_id())
            creating_file = '{}/{}'.format(creating_dir, CREATING_NAME)
            if not os.path.exists(creating_dir):
                os.makedirs(creating_dir)
            with open(creating_file, 'w') as f:
                f.write(self.identifying_info)
            r.index.add([creating_file])
            # if we are not initialized, trigger another travis build
            if initialized:
                r.index.commit('cluster build started [skip ci]')
            else:
                r.index.commit('cluster build started')
        tfvars_file = '{}/terraform_inputs/.{}.tfvars'.format(r.working_dir, self.platform)
        with TemporaryDirectory() as temp_dir:
            temp_dir = str(Path(temp_dir).resolve())
            self._temp_deploy_dir = temp_dir
            sh_env = os.environ.copy()
            sh_env['TF_VAR_private_key_file'] = self.private_key_file
            sh_env['TF_VAR_public_key_file'] = '{}.pub'.format(
                self.private_key_file)
            sh_env['TF_VAR_keypair_override'] = 'icp-cicd-pipeline-keypair'
            sh_env['TF_VAR_instance_name'] = INSTANCE_NAME

            exception = None
            tries = 1
            try:
                if 'openstack' in self.platform:
                    run('make -s deploy:openstack OPENSTACK_DEPLOY_DIR={} OPENSTACK_TERRAFORM_VARS_FILE={}'.format(temp_dir, tfvars_file),
                        shell=True,
                        check=True,
                        env=sh_env)
                elif 'openshift-aws' in self.platform:
                    run('make -s deploy:openshift:aws OPENSHIFT_AWS_DEPLOY_DIR={} OPENSHIFT_AWS_TERRAFORM_VARS_FILE={}'.format(temp_dir, tfvars_file),
                        shell=True,
                        check=True,
                        env=sh_env)
                elif 'openshift4-aws' in self.platform:
                    if 'travis' in self.platform:
                        run('sed -i="" -e "s|__CLUSTER_NAME__|pool-os4-{}|g" {}'.format(creating_id(),tfvars_file),
                            shell=True,
                            check=True,
                            env=sh_env)
                    run('sed -i="" -e "s|__W3_EMAIL__|$TF_VAR_user_name|g" -e "s|__AWS_ACCESS_KEY__|$TF_VAR_aws_access_key|g" -e "s|__AWS_SECRET_ACCESS_KEY__|$TF_VAR_aws_secret_key|g" -e "s|__RH_SUBSCRIPTION_USERNAME__|$TF_VAR_rhel_subscription_username|g" -e "s|__RH_SUBSCRIPTION_PASSWORD__|$TF_VAR_rhel_subscription_password|g" -e "s|__OCP__PULL_SECRET_FILE__|$OCP_PULL_SECRET_FILE|g" -e "s|__PRIVATE_KEY_FILE__|$PRIVATE_KEY_FILE|g" -e "s|__PUBLIC_KEY_FILE__|$PUBLIC_KEY_FILE|g" -e "s|__EDITION__|$EDITION|g" -e "s|__FIXPACK__|$FIXPACK|g" -e "s|__VERSION__|$VERSION|g" -e "s|__REPO__|$DEPLOY_REPO|g" -e "s|__ARTIFACTORY_USER__|$ARTIFACTORY_USER|g" -e "s|__ARTIFACTORY_API_KEY__|$ARTIFACTORY_TOKEN|g" {}'.format(tfvars_file),
                        shell=True,
                        check=True,
                        env=sh_env)
                    run('make -s deploy:openshift4:aws OPENSHIFT_4_AWS_DEPLOY_DIR={} OPENSHIFT_4_AWS_TERRAFORM_VARS_FILE={}'.format(temp_dir, tfvars_file),
                        shell=True,
                        check=True,
                        env=sh_env)
                elif 'aws' in self.platform:
                    run('make -s deploy:aws AWS_DEPLOY_DIR={} AWS_TERRAFORM_VARS_FILE={}'.format(temp_dir, tfvars_file),
                        shell=True,
                        check=True,
                        env=sh_env)
                else:
                    raise ClusterException('No rule in place to handle the tfvars file passed: {}. Ensure that you are trying to deploy a supported cluster.'.format(tfvars_file))
                exception = None
            except CalledProcessError as e:
                exception = e
                module_logger.error('Cluster failed to create successfully; retries left: {}'.format(self.max_tries - tries))
            
            while tries < self.max_tries:
                tries += 1
                if exception is None:
                    # If we don't have an exception, there is no need to retry.
                    break
                try:
                    run('make -s terraform:apply TERRAFORM_DIR={} TERRAFORM_VARS_FILE={}'.format(temp_dir, tfvars_file),
                            shell=True,
                            check=True,
                            env=sh_env)
                    exception = None
                except CalledProcessError as e:
                    exception = e
                    module_logger.error('Cluster failed to create successfully; retries left: {}'.format(self.max_tries - tries))
            
            if exception is not None:
                module_logger.error('Cluster failed to create successfully; deleting cluster')
                module_logger.error(exception)
                if not self.allow_failed_create:
                    self.delete(terraform_dir=temp_dir, skip_ci=False)
                    return
                else:
                    module_logger.info('Deletion skipped; we are allowing failed clusters')
            self.name = self._get_terraform_output(
                'cluster-name', self.deploy_dir, length_of_output=1)
            if self.name == '':
                self.name = None
                self.delete(terraform_dir=temp_dir)
                return
            # If there is a duplicate directory, shutil.copytree will fail with FileExistsError
            # TODO determine what to do when this happens
            try:
                with self.git_repo as r:
                    dest_repo_dir = '{}/{}'.format(self.cluster_dir, DEPLOY_DIR_NAME)
                    if 'openshift4-aws' in self.platform:
                        src_repo_tfstate_dir='{}/aws-ipi'.format(self.deploy_dir, DEPLOY_DIR_NAME)
                        dest_repo_tfstate_dir='{}/aws-ipi'.format(dest_repo_dir, DEPLOY_DIR_NAME)
                    else:
                        src_repo_tfstate_dir=self.deploy_dir
                        dest_repo_tfstate_dir=dest_repo_dir
                    shutil.copytree(temp_dir, '{}'.format(dest_repo_dir),
                                    ignore=shutil.ignore_patterns('.git*', 'terraform.tfstate'))
                    # encrypt tfstate file, remove real one before pushing to github
                    run_cmd='openssl enc -aes-256-cbc -salt -in {}/terraform.tfstate -out {}/terraform.tfstate.enc -k {}'.format(src_repo_tfstate_dir, dest_repo_tfstate_dir, 'afro-donkey-seldom-waterfall-compute')
                    run(run_cmd, shell=True)
                    touch('{}/{}'.format(self.cluster_dir, AVAILABLE_NAME))
                    self._modify_modules_json(
                        key='Dir', value='{}/{}/{}'.format(self.cluster_directory, self.name, DEPLOY_DIR_NAME))
                    r.index.add([self.cluster_dir])
                    r.index.move([creating_file, '{}/source-job.md'.format(self.cluster_dir)])
                    r.index.commit(
                        'save cluster state {} [skip ci]'.format(self.name))
                    with with_cd(self.git_repo.git_dir):
                        run('pwd; git status', shell=True)
            except Exception as e:
                module_logger.error('Error trying to save the cluster. Deleting.')
                module_logger.error(e)
                self.delete(terraform_dir=temp_dir)
                raise e
        self._temp_deploy_dir = None

    def checkout(self, name=None):
        module_logger.debug('cluster.checkout() entry.  name: {} self.name: {} self._cluster_claimed: {}'.format(name, self.name, self._cluster_claimed))
        # determine if we already have a cluster reserved/checked out
        # if not, attempt to reserve a cluster
        if self._cluster_claimed:
            return
        with self.git_repo as r:
            if self.name == None:
                module_logger.debug('self.name is none; things are going to get random here')
                clusters_available = glob(
                    '{}/{}/**/{}'.format(self.git_repo.git_dir, self.cluster_directory, AVAILABLE_NAME))
                if len(clusters_available) == 0:
                    clusters_creating = len(glob(
                        '{}/{}/**/{}'.format(self.git_repo.git_dir, self.cluster_directory, CREATING_NAME)))
                    grammar = "are"
                    if clusters_creating == 1:
                        grammar = "is"
                    raise NoClustersAvailable(
                        'No clusters available in the {} clusterpool; {} {} currently under construction. Checkout failed.'.format(self.platform, clusters_creating, grammar))
                random_cluster = choice(clusters_available)
                self.name = os.path.basename(os.path.dirname(random_cluster))
            else:
                module_logger.debug('self.name is {}; picking that exact one'.format(self.name))
                random_cluster = '{}/{}/{}'.format(self.git_repo.git_dir, self.cluster_directory, self.name)
            module_logger.info('Checking out cluster {}.'.format(self.name))
            reservation_file = '{}/{}'.format(self.cluster_dir, IN_USE_NAME)
            with open(reservation_file, 'w') as f:
                f.write(self.identifying_info)
            r.index.add([reservation_file])
            r.index.remove(
                ['{}/{}'.format(self.cluster_dir, AVAILABLE_NAME)], r=True, working_tree=True)
            r.index.commit('reserve {} {}'.format(
                self.platform, self.name))
        self._cluster_claimed = True

    def configure(self, helm_config=None, kubectl_config=None, override_claim=False):
        # ensure that the modules file is configured properly
        try:
            self.is_alive()
        except ClusterException as e:
            module_logger.error('Exception contacting cluster {}: {}'.format(self.name, e))
            module_logger.info('Cluster dead, marking for cleanup.')
            self.put_back(trigger=CLEANUP_TRIGGER)
            raise ClusterDead(
                'The cluster is not responding. Please reserve another cluster and try again.')
        # TODO pull and check to see if file still exists. If not, reset state and prompt to reserve another
        #       This is to protect against clusters getting cleaned up underneath
        if helm_config is None:
            helm_config = os.path.join(os.path.expanduser('~'), '.helm')
        if kubectl_config is None:
            kubectl_config = os.path.join(os.path.expanduser('~'), '.kubectl')
        if not self._cluster_claimed and not override_claim:
            raise(
                ClusterNotInitialized('You have to check out a cluster before you can configure it!'))
        # The modules.json file has the wrong directory path in it
        #   We need to swap out for the actual path of the module on this machine
        if os.path.exists('build-harness'):
            with with_cd('build-harness'):
                self._execute_configure(kubectl_config, helm_config)
        else:
            self._execute_configure(kubectl_config, helm_config)

    def _execute_configure(self, kubectl_config, helm_config):
        if self.master_node == '':
            raise Exception(
                'Panic: the master IP address could not be pulled for the cluster:')
        commands = [
            'make kubectl:install',
            'make kubectl:config K8S_CLUSTER_NAME={} K8S_CLUSTER_MASTER_IP={} KUBECTL_SSH_PRIVATE_KEY={} KUBERNETES_CLUSTER_CONFIG_PATH={} K8S_CLUSTER_SSH_USER={}'.format(
                self.name, self.master_node, self.private_key_file, kubectl_config, self.ssh_user),
            'make helm:config K8S_CLUSTER_NAME={} K8S_CLUSTER_MASTER_IP={} HELM_SSH_PRIVATE_KEY={} HELM_CLUSTER_CONFIG_PATH={} K8S_CLUSTER_SSH_USER={}'.format(
                self.name, self.master_node, self.private_key_file, helm_config, self.ssh_user),
            'make helm:init'
        ]
        for c in commands:
            output = run(c, shell=True, stdout=PIPE, timeout=30)
            if output.returncode != 0:
                raise Exception('command {} exited with return code {}!\n{}'.format(
                    c, output.returncode, filter_out_make_commands(output.stdout)))

    def put_back(self, tainted=True, file_rm=IN_USE_NAME, trigger=RETURN_TRIGGER, override=False):
        if not self._cluster_claimed and not override:
            raise ClusterNotInitialized(
                'No cluster checked out; cluster cannot be returned')
        with self.git_repo as r:
            if tainted:
                # if we are tainted, we want to make sure that
                #   there is a change so the commit will be pushed and we also
                #   want to create this file before the directory is deleted
                random_file = '{}/{}_{}'.format(
                    self.cluster_dir, TAINTED_NAME, randint(1, 100000))
                touch(random_file)
                r.index.add([random_file])
            if os.path.isfile('{}/{}'.format(self.cluster_dir, file_rm)):
                r.index.remove(
                    ['{}/{}'.format(self.cluster_dir, file_rm)], r=True, working_tree=True)
            if tainted:
                r.index.commit('{} {} {}'.format(
                    trigger, self.platform, self.name))
            else:
                # put back the AVAILABLE_NAME file
                touch('{}/{}'.format(self.cluster_dir, AVAILABLE_NAME))
                r.index.add(['{}/{}'.format(self.cluster_dir, AVAILABLE_NAME)])
                r.index.commit('return untainted cluster [skip ci]')
        module_logger.info('Returning cluster {}.'.format(self.name))
        self._cluster_claimed = False

    def delete(self, terraform_dir=None, skip_ci=True):
        if (self.name is not None and not self.name.isdigit()) or terraform_dir is not None:
            # If we have an actual cluster, ensure that it is deleted
            self._delete_cluster(terraform_dir)

        with self.git_repo as r:
            # check for creating files and delete as necessary
            platform_dir = '{}/{}'.format(r.working_dir, self.cluster_directory)
            if os.path.exists('{}/{}'.format(platform_dir, creating_id())):
                r.index.remove(
                    ['{}/{}'.format(platform_dir, creating_id())], r=True, working_tree=True, f=True)

                commit_message = 'cluster build failed for {} {}'.format(self.platform, creating_id())
                if skip_ci:
                    commit_message += ' [skip ci]'
                r.index.commit(commit_message)
            # check for cluster name files and delete as necessary
            elif self.name is not None:
                r.index.remove([self.cluster_dir], r=True,
                               working_tree=True, f=True)
                commit_message = 'deleted {} {}'.format(self.platform, self.name)
                if skip_ci:
                    commit_message += ' [skip ci]'
                r.index.commit(commit_message)

    def _delete_cluster(self, terraform_dir=None):
        module_logger.debug('cluster._delete_cluster() entry, terraform_dir={} self.deploy_dir={} self.platform={}'.format(terraform_dir, self.deploy_dir, self.platform)) 
        if terraform_dir is None:
            if self.name is None:
                raise ClusterNotInitialized(
                    'Cannot delete if no cluster name is specified.')
            terraform_dir = self.deploy_dir
        if 'openshift4-aws' in self.platform:
            deploy_dir_suffix='/aws-ipi'
        else:
            deploy_dir_suffix=''
        tfstate_file='{}{}/terraform.tfstate'.format(terraform_dir, deploy_dir_suffix)
        module_logger.debug('tfstate_file={} exists={}'.format(tfstate_file,os.path.isfile(tfstate_file))) 
        self._modify_modules_json()
        tfvars_file = '{}/terraform_inputs/.{}.tfvars'.format(
            self.git_repo.git_dir, self.platform)
        sh_env = os.environ.copy()
        sh_env['TF_VAR_private_key_file'] = self.private_key_file
        sh_env['TF_VAR_public_key_file'] = '{}.pub'.format(
            self.private_key_file)

        with with_cd(self.git_repo.git_dir):
            with TemporaryDirectory() as temp_tfstate_dir:
                temp_tfstate_dir = str(Path(temp_tfstate_dir).resolve())
                if 'openshift4-aws' in self.platform:
                    run('sed -i="" -e "s|__W3_EMAIL__|$TF_VAR_user_name|g" -e "s|__AWS_ACCESS_KEY__|$TF_VAR_aws_access_key|g" -e "s|__AWS_SECRET_ACCESS_KEY__|$TF_VAR_aws_secret_key|g" -e "s|__RH_SUBSCRIPTION_USERNAME__|$TF_VAR_rhel_subscription_username|g" -e "s|__RH_SUBSCRIPTION_PASSWORD__|$TF_VAR_rhel_subscription_password|g" -e "s|__OCP__PULL_SECRET_FILE__|$OCP_PULL_SECRET_FILE|g" -e "s|__PRIVATE_KEY_FILE__|$PRIVATE_KEY_FILE|g" -e "s|__PUBLIC_KEY_FILE__|$PUBLIC_KEY_FILE|g" -e "s|__EDITION__|$EDITION|g" -e "s|__FIXPACK__|$FIXPACK|g" -e "s|__VERSION__|$VERSION|g" -e "s|__REPO__|$DEPLOY_REPO|g" -e "s|__ARTIFACTORY_USER__|$ARTIFACTORY_USER|g" -e "s|__ARTIFACTORY_API_KEY__|$ARTIFACTORY_TOKEN|g" {}'.format(tfvars_file),
                        shell=True,
                        check=True,
                        env=sh_env)
                    if not os.path.isfile('pull-secret'):
                        run('openssl aes-256-cbc -K ${} -iv ${} -in pull-secret.enc -out pull-secret -d'.format(os.environ['OCP_PULL_SECRET_FILE_VAR_KEY'], os.environ['OCP_PULL_SECRET_FILE_VAR_IV']),
                            shell=True,
                            check=True,
                            env=sh_env)
                    if 'travis' in self.platform:
                        module_logger.debug('found openshift4-aws-travis in self.platform; running sed on __CLUSTER_NAME__ of {}.'.format(tfvars_file))
                        run('sed -i="" -e "s|__CLUSTER_NAME__|{}|g" {}'.format(self._get_terraform_output('cluster-name', self.deploy_dir, length_of_output=1),tfvars_file),
                            shell=True,
                            check=True,
                            env=sh_env)
                    if os.path.isfile('{}.enc'.format(tfstate_file)):
                        decrypted_tfstate_file = '{}/terraform.tfstate'.format(temp_tfstate_dir)
                        run_cmd='openssl enc -aes-256-cbc -d -in {}.enc -out {} -k {}'.format(tfstate_file, decrypted_tfstate_file, 'afro-donkey-seldom-waterfall-compute')
                        run(run_cmd, shell=True)
                    else:
                        decrypted_tfstate_file = tfstate_file
                    # aws-ipi needs this abomination because of its directory hierarchy; else cluster destroys fail
                    destroyer_cmd='cd ..; make -s terraform:destroy TERRAFORM_DIR={}/aws-ipi TERRAFORM_VARS_FILE={} TERRAFORM_STATE_FILE={}'.format(self.deploy_dir, tfvars_file, decrypted_tfstate_file)
                else:
                    destroyer_cmd='make -s terraform:destroy TERRAFORM_DIR={} TERRAFORM_VARS_FILE={} TERRAFORM_STATE_FILE={}'.format(self.deploy_dir, tfvars_file, tfstate_file)

                module_logger.debug('terraform invocation:\n    {}'.format(destroyer_cmd))
                output = run(
                    destroyer_cmd, 
                    shell=True, 
                    env=sh_env)
                if output.returncode != 0:
                    module_logger.info(
                        'Destroy completed with errors, but everything is likely cleaned up.')

    def is_alive(self, timeout=30):
        if not os.path.exists(self.deploy_dir):
            raise ClusterNotFound(
                'Cluster deploy directory "{}" could not be found'.format(self.deploy_dir))
        # if we do not have an exception raised, we are good!
        self.execute_shell_command()
        return True

    def execute_shell_command(self, commands=list(), timeout=30, ssh_env=dict()):
        self._modify_modules_json()
        
        ssh = paramiko.SSHClient()
        ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        try:
            ssh.connect(self.master_node, username=self.ssh_user,
                        key_filename=self.private_key_file, look_for_keys=False, timeout=timeout)
            for c in commands:
                _, stdout, _ = ssh.exec_command(c, timeout=timeout, environment=ssh_env)
                print(stdout.read().decode('utf-8').rstrip('\n'))
        except sock_timeout:
            raise ClusterDead('Socket timeout; marking cluster as dead')
        except paramiko.ssh_exception.SSHException as e:
            raise ClusterDead('Error with ssh tunnel:\n{}'.format(str(e)))
        finally:
            ssh.close()
